package server

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Z-lab-boop/harnessscope/internal/model"
)

func httpService(t *testing.T, scan ScanFunc) *Service {
	t.Helper()
	root := t.TempDir()
	if scan == nil {
		scan = func(context.Context) (model.ScanResult, error) { return model.ScanResult{}, nil }
	}
	s, err := NewService(ServiceConfig{Workspace: root, HomeDir: filepath.Dir(root), AppDataDir: filepath.Join(root, "data"), Scan: scan})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestHTTPLaunchServesReadyProtectedStateAndExactAssets(t *testing.T) {
	var logs bytes.Buffer
	i, err := Start(context.Background(), HTTPConfig{Service: httpService(t, nil), Logger: log.New(&logs, "", 0)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = i.Close(context.Background()) })
	u, err := url.Parse(i.URL())
	if err != nil {
		t.Fatal(err)
	}
	fragment, _ := url.ParseQuery(u.Fragment)
	token := fragment.Get("token")
	if u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.Port() == "0" || u.RawQuery != "" || len(token) != 43 {
		t.Fatalf("invalid launch URL shape: scheme=%s host=%s query=%q token length=%d", u.Scheme, u.Host, u.RawQuery, len(token))
	}
	u.Fragment = ""
	client := &http.Client{Timeout: 3 * time.Second}
	defer client.CloseIdleConnections()
	for _, tc := range []struct {
		path, mime string
		status     int
	}{
		{"/", "text/html; charset=utf-8", 200},
		{"/assets/dashboard.js", "text/javascript; charset=utf-8", 200},
		{"/assets/dashboard.css", "text/css; charset=utf-8", 200},
		{"/unknown", "", 404}, {"/assets/", "", 404}, {"/assets/index.html", "", 404},
		{"/assets/../index.html", "", 404}, {"//", "", 404},
		{"/api/v1/state", "application/json", 401}, {"/api/v1/unknown", "application/json", 401},
	} {
		t.Run(tc.path, func(t *testing.T) {
			resp, err := client.Get(strings.TrimSuffix(u.String(), "/") + tc.path)
			if err != nil {
				t.Fatal(err)
			}
			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != tc.status || (tc.mime != "" && resp.Header.Get("Content-Type") != tc.mime) {
				t.Fatalf("status=%d MIME=%q", resp.StatusCode, resp.Header.Get("Content-Type"))
			}
			if strings.Contains(string(body), token) {
				t.Fatal("token leaked in response")
			}
			if resp.Header.Get("Content-Security-Policy") == "" || resp.Header.Get("Cache-Control") != "no-store" {
				t.Fatal("security headers missing")
			}
		})
	}
	req, _ := http.NewRequest("GET", strings.TrimSuffix(u.String(), "/")+"/api/v1/state", nil)
	req.Header.Set("X-HarnessScope-Token", token)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var state DashboardState
	err = json.NewDecoder(resp.Body).Decode(&state)
	resp.Body.Close()
	if err != nil || resp.StatusCode != 200 || state.Revision != 1 {
		t.Fatalf("not ready: status=%d revision=%d err=%v", resp.StatusCode, state.Revision, err)
	}
	if err := i.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(logs.String(), token) {
		t.Fatal("token leaked to logs")
	}
}

func TestHTTPConfiguredPortCancellationAndRepeatedClose(t *testing.T) {
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := probe.Addr().(*net.TCPAddr).Port
	probe.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	i, err := Start(ctx, HTTPConfig{Port: port, Service: httpService(t, nil)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = i.Close(context.Background()) })
	u, _ := url.Parse(i.URL())
	if u.Host != net.JoinHostPort("127.0.0.1", strconv.Itoa(port)) {
		t.Fatal("did not bind requested loopback port")
	}
	cancel()
	finished := make(chan error, 1)
	go func() { finished <- i.Wait() }()
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("cancellation did not stop listener")
	}
	for n := 0; n < 2; n++ {
		if err := i.Close(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	probe, err = net.Listen("tcp", u.Host)
	if err != nil {
		t.Fatalf("listener was not released: %v", err)
	}
	probe.Close()
}

func TestHTTPRejectsInvalidStartupAndClosesListenerOnEntropyFailure(t *testing.T) {
	s := httpService(t, nil)
	for _, host := range []string{"0.0.0.0", "localhost", "192.0.2.1", "::"} {
		if i, err := Start(context.Background(), HTTPConfig{Host: host, Service: s}); err == nil {
			i.Close(context.Background())
			t.Fatalf("accepted %s", host)
		}
	}
	for _, config := range []HTTPConfig{{Service: nil}, {Service: &Service{}}, {Service: s, Port: -1}, {Service: s, Port: 65536}} {
		if i, err := Start(context.Background(), config); err == nil {
			i.Close(context.Background())
			t.Fatal("accepted invalid startup")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if i, err := Start(ctx, HTTPConfig{Service: s}); !errors.Is(err, context.Canceled) {
		if i != nil {
			i.Close(context.Background())
		}
		t.Fatalf("canceled start: %v", err)
	}
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := probe.Addr().String()
	port := probe.Addr().(*net.TCPAddr).Port
	if i, err := Start(context.Background(), HTTPConfig{Port: port, Service: s}); err == nil {
		i.Close(context.Background())
		t.Fatal("accepted occupied port")
	}
	probe.Close()
	if i, err := Start(context.Background(), HTTPConfig{Port: port, Service: s, Random: strings.NewReader("")}); err == nil {
		i.Close(context.Background())
		t.Fatal("accepted failed entropy")
	}
	probe, err = net.Listen("tcp", address)
	if err != nil {
		t.Fatalf("entropy failure leaked listener: %v", err)
	}
	probe.Close()
}

func TestHTTPInjectsVersionAndSanitizedLogger(t *testing.T) {
	var logs bytes.Buffer
	var scanError atomic.Value
	s := httpService(t, func(context.Context) (model.ScanResult, error) {
		if err, ok := scanError.Load().(error); ok {
			return model.ScanResult{}, err
		}
		return model.ScanResult{}, nil
	})
	i, err := Start(context.Background(), HTTPConfig{Service: s, Logger: log.New(&logs, "", 0), ToolVersion: "test-build-8"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = i.Close(context.Background()) })
	u, _ := url.Parse(i.URL())
	fragment, _ := url.ParseQuery(u.Fragment)
	token := fragment.Get("token")
	u.Fragment = ""
	base := strings.TrimSuffix(u.String(), "/")
	client := &http.Client{Timeout: 3 * time.Second}
	defer client.CloseIdleConnections()
	post := func(path string) *http.Response {
		t.Helper()
		req, _ := http.NewRequest("POST", base+path, strings.NewReader(`{"revision":1}`))
		req.Header.Set("X-HarnessScope-Token", token)
		req.Header.Set("Origin", base)
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}
	resp := post("/api/v1/export")
	data, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("export status=%d", resp.StatusCode)
	}
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range z.File {
		if f.Name == "manifest.json" {
			r, _ := f.Open()
			manifest, _ := io.ReadAll(r)
			r.Close()
			found = bytes.Contains(manifest, []byte("test-build-8"))
		}
	}
	if !found {
		t.Fatal("export lost injected tool version")
	}
	scanError.Store(errors.New("scan failed " + token + " /private/secret/file"))
	resp = post("/api/v1/rescan")
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 500 {
		t.Fatalf("rescan status=%d", resp.StatusCode)
	}
	i.Close(context.Background())
	if logs.Len() == 0 || strings.Contains(logs.String(), token) || strings.Contains(logs.String(), "/private/secret") {
		t.Fatal("logger missing or leaked sensitive values")
	}
}

func TestHTTPCancellationReleasesActiveRequest(t *testing.T) {
	entered := make(chan struct{})
	var scans atomic.Int32
	s := httpService(t, func(ctx context.Context) (model.ScanResult, error) {
		if scans.Add(1) == 1 {
			return model.ScanResult{}, nil
		}
		close(entered)
		<-ctx.Done()
		return model.ScanResult{}, ctx.Err()
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	i, err := Start(ctx, HTTPConfig{Service: s})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = i.Close(context.Background()) })
	u, _ := url.Parse(i.URL())
	fragment, _ := url.ParseQuery(u.Fragment)
	u.Fragment = ""
	base := strings.TrimSuffix(u.String(), "/")
	req, _ := http.NewRequest("POST", base+"/api/v1/rescan", strings.NewReader(`{"revision":1}`))
	req.Header.Set("X-HarnessScope-Token", fragment.Get("token"))
	req.Header.Set("Origin", base)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 3 * time.Second}
	defer client.CloseIdleConnections()
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		resp, err := client.Do(req)
		if err == nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
	}()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("request did not reach scan")
	}
	cancel()
	waited := make(chan error, 1)
	go func() { waited <- i.Wait() }()
	select {
	case err := <-waited:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("active request prevented shutdown")
	}
	select {
	case <-finished:
	case <-time.After(3 * time.Second):
		t.Fatal("request goroutine leaked")
	}
	if s.State().Revision != 1 {
		t.Fatal("canceled scan published state")
	}
}

func TestHTTPWaitDrainsMutationAfterForcedClose(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	var scans atomic.Int32
	s := httpService(t, func(context.Context) (model.ScanResult, error) {
		if scans.Add(1) == 1 {
			return model.ScanResult{}, nil
		}
		close(entered)
		<-release // Model a cancellation-proof transaction unwind.
		return model.ScanResult{}, context.Canceled
	})
	i, err := Start(context.Background(), HTTPConfig{Service: s})
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(i.URL())
	fragment, _ := url.ParseQuery(u.Fragment)
	u.Fragment = ""
	base := strings.TrimSuffix(u.String(), "/")
	req, _ := http.NewRequest("POST", base+"/api/v1/rescan", strings.NewReader(`{"revision":1}`))
	req.Header.Set("X-HarnessScope-Token", fragment.Get("token"))
	req.Header.Set("Origin", base)
	req.Header.Set("Content-Type", "application/json")
	requestDone := make(chan struct{})
	go func() {
		defer close(requestDone)
		response, err := http.DefaultClient.Do(req)
		if err == nil {
			io.Copy(io.Discard, response.Body)
			response.Body.Close()
		}
	}()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("mutation did not start")
	}
	closeCtx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if err := i.Close(closeCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("forced close error=%v", err)
	}
	waited := make(chan error, 1)
	go func() { waited <- i.Wait() }()
	select {
	case err := <-waited:
		t.Fatalf("Wait returned before mutation drained: %v", err)
	case <-time.After(80 * time.Millisecond):
	}
	close(release)
	select {
	case err := <-waited:
		if err != nil && !errors.Is(err, context.DeadlineExceeded) {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Wait deadlocked after mutation completed")
	}
	for n := 0; n < 4; n++ {
		if err := i.Close(context.Background()); err != nil && !errors.Is(err, context.DeadlineExceeded) {
			t.Fatal(err)
		}
	}
	select {
	case <-requestDone:
	case <-time.After(3 * time.Second):
		t.Fatal("request goroutine leaked")
	}
}
