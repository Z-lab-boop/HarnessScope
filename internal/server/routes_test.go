package server

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Z-lab-boop/harnessscope/internal/model"
	"github.com/Z-lab-boop/harnessscope/internal/snapshots"
)

func routeRequest(t *testing.T, h http.Handler, session Session, method, target, body string, change func(*http.Request)) *httptest.ResponseRecorder {
	t.Helper()
	r := localRequest(method, "127.0.0.1:8123", body)
	u, err := r.URL.Parse(target)
	if err != nil {
		t.Fatal(err)
	}
	r.URL = u
	r.Header.Set("X-HarnessScope-Token", session.Token())
	r.Header.Set("Origin", "http://127.0.0.1:8123")
	r.Header.Set("Content-Type", "application/json")
	if change != nil {
		change(r)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	assertSecurityHeaders(t, w)
	return w
}

func routeJSON(t *testing.T, w *httptest.ResponseRecorder, status int, target any) {
	t.Helper()
	if w.Code != status || w.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("status/type = %d %q, want %d JSON; body=%s", w.Code, w.Header().Get("Content-Type"), status, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), target); err != nil {
		t.Fatal(err)
	}
}

func TestRoutesReadContracts(t *testing.T) {
	root := t.TempDir()
	s, err := NewService(ServiceConfig{Workspace: root, HomeDir: filepath.Dir(root), AppDataDir: filepath.Join(root, "data"), Scan: func(context.Context) (model.ScanResult, error) {
		return model.ScanResult{SchemaVersion: model.ReportSchemaVersion, Analysis: model.Analysis{
			Clients: []model.ClientResult{{ID: "codex"}, {ID: "claude"}, {ID: "other"}},
			Graph: model.Graph{Nodes: []model.ConfigNode{
				{ID: "node-a", Client: "codex", Type: model.NodeEffective, DisplayName: "model", Origins: []model.Origin{{SourceID: "source-a", LogicalPath: filepath.Join(root, "config")}}, Attributes: map[string]model.SafeValue{"value": {Display: "one"}}},
				{ID: "node-b", Client: "claude", Type: model.NodeEffective, DisplayName: "model", Attributes: map[string]model.SafeValue{"value": {Display: "two"}}},
				{ID: "node-c", Client: "other", Type: model.NodeEffective, DisplayName: "irrelevant"},
			}, Edges: []model.Edge{{ID: "edge-a", From: "node-a", To: "source-a"}}},
		}}, nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	session := fixedSession(t)
	h := NewHandler(s, session)
	var state DashboardState
	routeJSON(t, routeRequest(t, h, session, "GET", "/api/v1/state", "", nil), 200, &state)
	if state.Revision != 1 || strings.Contains(fmt.Sprint(state), root) {
		t.Fatal("unsafe or incomplete state")
	}
	var detail ExplainResponse
	routeJSON(t, routeRequest(t, h, session, "GET", "/api/v1/explain?id=node-a", "", nil), 200, &detail)
	if detail.Node.ID != "node-a" || len(detail.Node.Origins) != 1 || len(detail.Edges) != 1 {
		t.Fatalf("missing provenance: %+v", detail)
	}
	var comparison ComparisonResponse
	routeJSON(t, routeRequest(t, h, session, "GET", "/api/v1/compare?left=codex&right=claude", "", nil), 200, &comparison)
	if len(comparison.Rows) != 1 || comparison.Rows[0].Status != "different" || comparison.Rows[0].Name != "model" {
		t.Fatalf("wrong selected-client matrix: %+v", comparison)
	}
	for _, path := range []string{"/api/v1/backups", "/api/v1/snapshots"} {
		var items []any
		w := routeRequest(t, h, session, "GET", path, "", nil)
		routeJSON(t, w, 200, &items)
		if items == nil || len(items) != 0 {
			t.Fatalf("expected empty array: %s", w.Body.String())
		}
	}
	for _, tc := range []struct {
		path   string
		status int
	}{
		{"/api/v1/explain?id=model", 404}, {"/api/v1/explain?id=node", 404},
		{"/api/v1/explain", 400}, {"/api/v1/explain?id=../../config", 400},
		{"/api/v1/explain?id=node-a&id=node-b", 400}, {"/api/v1/state?path=/tmp", 400},
		{"/api/v1/compare?left=codex&right=missing", 404}, {"/api/v1/compare?left=codex&right=/tmp", 400},
		{"/api/v1/compare?left=codex", 400}, {"/api/v1/unknown", 404},
	} {
		var e APIError
		routeJSON(t, routeRequest(t, h, session, "GET", tc.path, "", nil), tc.status, &e)
		if e.Code == "" || e.Message == "" || e.Details == nil {
			t.Fatal("unstable error envelope")
		}
	}
}

func TestRoutesFixPrivacyRoundTrip(t *testing.T) {
	s, paths, _ := serviceFixture(t, 1, nil)
	original := []byte("PRIVATE_INSTRUCTION_CANARY\nPRIVATE_INSTRUCTION_CANARY\napi_key=sk-canary-private-value-123456789\n")
	if err := os.WriteFile(paths[0], original, 0o640); err != nil {
		t.Fatal(err)
	}
	session := fixedSession(t)
	instance, err := Start(context.Background(), HTTPConfig{Service: s, Random: bytes.NewReader(bytes.Repeat([]byte{0xfb}, 32))})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = instance.Close(context.Background()) })
	base, _, _ := strings.Cut(instance.URL(), "/#")
	client := &http.Client{Timeout: 3 * time.Second}
	defer client.CloseIdleConnections()
	request := func(method, path, body string, status int, target any) {
		r, err := http.NewRequest(method, base+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		r.Header.Set("X-HarnessScope-Token", session.Token())
		r.Header.Set("Origin", base)
		r.Header.Set("Content-Type", "application/json")
		response, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != status || response.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("%s: status=%d, want=%d", path, response.StatusCode, status)
		}
		if err := json.Unmarshal(data, target); err != nil {
			t.Fatal(err)
		}
		for _, private := range []string{"PRIVATE_INSTRUCTION_CANARY", "sk-canary-private-value", s.config.Workspace, string(original), session.Token()} {
			if bytes.Contains(data, []byte(private)) {
				t.Fatalf("public response leaked source content via %s", path)
			}
		}
	}
	var state DashboardState
	request("POST", "/api/v1/rescan", `{"revision":1}`, 200, &state)
	var plans []model.FixPlan
	request("POST", "/api/v1/fixes/plan", `{"revision":2}`, 200, &plans)
	if len(plans) != 1 || plans[0].Risk != model.RiskSafe || plans[0].Edits[0].RedactedPatch == "" {
		t.Fatal("missing SAFE preview")
	}
	id := plans[0].ID
	var apiErr APIError
	request("POST", "/api/v1/fixes/apply", fmt.Sprintf(`{"revision":1,"fix_ids":[%q]}`, id), 409, &apiErr)
	if apiErr.Code != "stale_revision" {
		t.Fatal(apiErr)
	}
	s.mu.Lock()
	s.public.FixPlans[0].Risk = model.RiskReview
	s.mu.Unlock()
	request("POST", "/api/v1/fixes/apply", fmt.Sprintf(`{"revision":2,"fix_ids":[%q]}`, id), 422, &apiErr)
	if apiErr.Code != "unsafe_fix" {
		t.Fatal(apiErr)
	}
	s.mu.Lock()
	s.public.FixPlans[0].Risk = model.RiskSafe
	s.mu.Unlock()
	request("POST", "/api/v1/fixes/apply", fmt.Sprintf(`{"revision":2,"fix_ids":[%q]}`, id), 200, &state)
	if state.Revision != 3 || len(state.FixPlans) != 0 || len(state.Backups) != 1 {
		t.Fatal("missing apply rescan/history")
	}
	data, _ := os.ReadFile(paths[0])
	if bytes.Equal(data, original) {
		t.Fatal("apply did not change file")
	}
	var backups []map[string]any
	request("GET", "/api/v1/backups", "", 200, &backups)
	if len(backups) != 1 || len(backups[0]) != 3 || backups[0]["file_count"] != float64(1) {
		t.Fatalf("unsafe backup summary: %+v", backups)
	}
	request("POST", "/api/v1/rescan", `{"revision":3}`, 200, &state)
	request("POST", "/api/v1/rollback", fmt.Sprintf(`{"revision":4,"backup_id":%q}`, backups[0]["id"]), 200, &state)
	data, _ = os.ReadFile(paths[0])
	info, _ := os.Stat(paths[0])
	if !bytes.Equal(data, original) || info.Mode().Perm() != 0o640 || state.Revision != 5 || len(state.FixPlans) != 1 {
		t.Fatal("rollback failed to restore bytes/mode/plan/revision")
	}
}

func TestRoutesMutationsAndDownload(t *testing.T) {
	s, paths, _ := serviceFixture(t, 1, nil)
	session := fixedSession(t)
	h := NewHandler(s, session)
	var plans []model.FixPlan
	routeJSON(t, routeRequest(t, h, session, "POST", "/api/v1/fixes/plan", `{"revision":1}`, nil), 200, &plans)
	if len(plans) != 1 {
		t.Fatal("missing fix")
	}
	var state DashboardState
	body := fmt.Sprintf(`{"revision":1,"fix_ids":[%q]}`, plans[0].ID)
	routeJSON(t, routeRequest(t, h, session, "POST", "/api/v1/fixes/apply", body, nil), 200, &state)
	if state.Revision != 2 || len(state.Backups) != 1 {
		t.Fatal("apply did not publish")
	}
	data, _ := os.ReadFile(paths[0])
	if string(data) != "keep\n" {
		t.Fatal("fix not applied")
	}
	body = fmt.Sprintf(`{"revision":2,"backup_id":%q}`, state.Backups[0].ID)
	routeJSON(t, routeRequest(t, h, session, "POST", "/api/v1/rollback", body, nil), 200, &state)
	if state.Revision != 3 {
		t.Fatal("rollback revision")
	}
	data, _ = os.ReadFile(paths[0])
	if string(data) != "keep\nkeep\n" {
		t.Fatal("backup not restored")
	}
	var meta map[string]any
	routeJSON(t, routeRequest(t, h, session, "POST", "/api/v1/snapshots", `{"revision":3,"name":"base"}`, nil), 201, &meta)
	if meta["name"] != "base" || meta["path"] != nil {
		t.Fatal("unsafe snapshot metadata")
	}
	var items []map[string]any
	routeJSON(t, routeRequest(t, h, session, "GET", "/api/v1/snapshots", "", nil), 200, &items)
	if len(items) != 1 || items[0]["name"] != "base" || items[0]["path"] != nil {
		t.Fatal("snapshot not listed safely")
	}
	routeJSON(t, routeRequest(t, h, session, "POST", "/api/v1/drift", `{"revision":3,"baseline":"base"}`, nil), 200, &state)
	if state.Revision != 4 || state.Drift == nil || state.Drift.Baseline != "base" {
		t.Fatal("drift not published")
	}
	w := routeRequest(t, h, session, "POST", "/api/v1/export", `{"revision":4,"baseline":"base"}`, nil)
	if w.Code != 200 || w.Header().Get("Content-Type") != "application/zip" || w.Header().Get("Content-Disposition") != `attachment; filename="harnessscope-diagnostics.zip"` {
		t.Fatalf("bad download: %d %v %s", w.Code, w.Header(), w.Body.String())
	}
	zr, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, file := range zr.File {
		if file.Name == "drift.json" {
			found = true
		}
		r, _ := file.Open()
		data, _ := io.ReadAll(r)
		r.Close()
		if bytes.Contains(data, []byte(s.config.Workspace)) {
			t.Fatal("path leaked in ZIP")
		}
	}
	if !found || s.State().Revision != 4 {
		t.Fatal("export changed state or omitted active drift")
	}
	routeJSON(t, routeRequest(t, h, session, "POST", "/api/v1/rescan", `{"revision":4}`, nil), 200, &state)
	if state.Revision != 5 {
		t.Fatal("rescan did not publish")
	}
}

func TestRoutesStrictBodiesAndErrors(t *testing.T) {
	s, _, calls := serviceFixture(t, 1, nil)
	session := fixedSession(t)
	h := NewHandler(s, session)
	for _, tc := range []struct {
		path, body string
		status     int
		code       string
	}{
		{"rescan", `{"revision":1,"path":"/tmp"}`, 400, "invalid_request"},
		{"rescan", `{"revision":1} {}`, 400, "invalid_request"},
		{"rescan", `{"revision":1} rubbish`, 400, "invalid_request"},
		{"rescan", `null`, 400, "invalid_request"}, {"rescan", `{}`, 400, "invalid_request"},
		{"rescan", `{"revision":1,"revision":1}`, 400, "invalid_request"},
		{"rescan", `{"Revision":1}`, 400, "invalid_request"},
		{"rescan", `{"revision":-1}`, 400, "invalid_request"},
		{"rescan", `{"revision":0}`, 409, "stale_revision"},
		{"fixes/plan", `{"revision":0}`, 409, "stale_revision"},
		{"fixes/apply", `{"revision":0,"fix_ids":["FIX-DUP-FFFFFFFF"]}`, 409, "stale_revision"},
		{"fixes/apply", `{"revision":1,"fix_ids":[]}`, 400, "invalid_request"},
		{"fixes/apply", `{"revision":1,"fix_ids":["../../file"]}`, 400, "invalid_request"},
		{"fixes/apply", `{"revision":1,"fix_ids":["FIX-DUP-FFFFFFFF"]}`, 404, "not_found"},
		{"rollback", `{"revision":1,"backup_id":"20261009T010203Z-0000000000000000"}`, 404, "not_found"},
		{"rollback", `{"revision":1,"backup_id":"/tmp"}`, 400, "invalid_request"},
		{"snapshots", `{"revision":1,"name":"../base"}`, 400, "invalid_request"},
		{"snapshots", `{"revision":0,"name":"base"}`, 409, "stale_revision"},
		{"drift", `{"revision":1,"baseline":"missing"}`, 404, "not_found"},
		{"drift", `{"revision":0,"baseline":"base"}`, 409, "stale_revision"},
		{"export", `{"revision":0}`, 409, "stale_revision"},
		{"export", `{"revision":1,"baseline":"missing"}`, 409, "stale_revision"},
		{"export", `{"revision":1,"baseline":"/tmp"}`, 400, "invalid_request"},
	} {
		t.Run(tc.path+tc.body, func(t *testing.T) {
			var e APIError
			routeJSON(t, routeRequest(t, h, session, "POST", "/api/v1/"+tc.path, tc.body, nil), tc.status, &e)
			if e.Code != tc.code || e.Details == nil {
				t.Fatalf("wrong error: %+v", e)
			}
		})
	}
	if calls.Load() != 1 || s.State().Revision != 1 {
		t.Fatal("rejected request scanned or published")
	}
	s.mu.Lock()
	s.public.FixPlans[0].Risk = model.RiskReview
	s.mu.Unlock()
	var e APIError
	routeJSON(t, routeRequest(t, h, session, "POST", "/api/v1/fixes/apply", fmt.Sprintf(`{"revision":1,"fix_ids":[%q]}`, s.State().FixPlans[0].ID), nil), 422, &e)
	if e.Code != "unsafe_fix" {
		t.Fatal(e)
	}
}

func TestRoutesAuthMethodsAndSecurityErrors(t *testing.T) {
	s, _, _ := serviceFixture(t, 0, nil)
	session := fixedSession(t)
	h := NewHandler(s, session)
	for _, tc := range []struct{ method, path, body string }{
		{"GET", "state", ""}, {"GET", "explain?id=x", ""}, {"GET", "compare?left=x&right=y", ""}, {"GET", "backups", ""}, {"GET", "snapshots", ""},
		{"POST", "rescan", `{"revision":1}`}, {"POST", "fixes/plan", `{"revision":1}`}, {"POST", "fixes/apply", `{}`}, {"POST", "rollback", `{}`}, {"POST", "snapshots", `{}`}, {"POST", "drift", `{}`}, {"POST", "export", `{}`}, {"GET", "unknown", ""},
	} {
		var e APIError
		routeJSON(t, routeRequest(t, h, session, tc.method, "/api/v1/"+tc.path, tc.body, func(r *http.Request) { r.Header.Del("X-HarnessScope-Token") }), 401, &e)
		wrong := "GET"
		if tc.method == "GET" {
			wrong = "POST"
		}
		if tc.path == "snapshots" {
			wrong = "PUT"
		}
		if tc.path == "unknown" {
			continue
		}
		routeJSON(t, routeRequest(t, h, session, wrong, "/api/v1/"+tc.path, `{}`, nil), 405, &e)
	}
	for _, tc := range []struct {
		change func(*http.Request)
		status int
	}{
		{func(r *http.Request) { r.Header.Set("Origin", "http://evil.example") }, 403},
		{func(r *http.Request) { r.Host = "evil.example" }, 403},
		{func(r *http.Request) { r.Header.Set("Content-Type", "text/plain") }, 415},
		{func(r *http.Request) {
			r.ContentLength = -1
			r.Body = io.NopCloser(strings.NewReader(strings.Repeat(" ", (1<<20)+1)))
		}, 413},
	} {
		var e APIError
		routeJSON(t, routeRequest(t, h, session, "POST", "/api/v1/rescan", `{}`, tc.change), tc.status, &e)
		if e.Code == "" || e.Details == nil {
			t.Fatal("security envelope")
		}
	}
	var e APIError
	routeJSON(t, routeRequest(t, h, session, "HEAD", "/api/v1/state", "", nil), 405, &e)
}

func TestRoutesInternalErrorRedactsLogAndResponse(t *testing.T) {
	session := fixedSession(t)
	secret := "sk-abcdefghijklmnopqrstuvwxyz123456"
	s, _, _ := serviceFixture(t, 0, func(n int) error {
		if n > 1 {
			return errors.New("scan failure /private/hidden/user/file token=" + session.Token() + " " + secret)
		}
		return nil
	})
	var logs bytes.Buffer
	h := newHandler(s, session, log.New(&logs, "", 0), "0.2.0")
	var e APIError
	w := routeRequest(t, h, session, "POST", "/api/v1/rescan", `{"revision":1}`, nil)
	routeJSON(t, w, 500, &e)
	if e.Code != "internal_error" || e.Message != "An internal error occurred." || logs.Len() == 0 {
		t.Fatalf("error handling: %+v %s", e, logs.String())
	}
	for _, canary := range []string{secret, session.Token(), "/private/hidden/user/file"} {
		if strings.Contains(w.Body.String()+logs.String(), canary) {
			t.Fatal("error leaked sensitive content")
		}
	}
	if strings.Contains(w.Body.String(), "scan failure") {
		t.Fatal("internal message exposed")
	}
}

func TestRoutesEveryMutationUsesStrictDecoder(t *testing.T) {
	s, _, calls := serviceFixture(t, 0, nil)
	session := fixedSession(t)
	h := NewHandler(s, session)
	for _, path := range []string{"rescan", "fixes/plan", "fixes/apply", "rollback", "snapshots", "drift", "export"} {
		for _, body := range []string{`{"revision":1,"path":"/private/canary"}`, `{"revision":1} {}`, `{"revision":null}`, `{"revision":1.2}`, `[]`, ``} {
			var e APIError
			w := routeRequest(t, h, session, "POST", "/api/v1/"+path, body, nil)
			routeJSON(t, w, 400, &e)
			if e.Code != "invalid_request" || strings.Contains(w.Body.String(), "canary") {
				t.Fatal("invalid body leaked or accepted")
			}
		}
	}
	if calls.Load() != 1 || s.State().Revision != 1 {
		t.Fatal("malformed body caused work")
	}
	if _, err := os.Stat(s.config.AppDataDir); !os.IsNotExist(err) {
		t.Fatal("malformed request created files")
	}
	var e APIError
	routeJSON(t, routeRequest(t, h, session, "GET", "/api/v1/state", `{"path":"/private/canary"}`, nil), 400, &e)
	// Legal trailing whitespace at the exact byte cap remains a single value.
	body := `{"revision":1}`
	var state DashboardState
	routeJSON(t, routeRequest(t, h, session, "POST", "/api/v1/rescan", body+strings.Repeat(" ", (1<<20)-len(body)), nil), 200, &state)
	if state.Revision != 2 {
		t.Fatal("body limit rejected legal boundary")
	}
	routeJSON(t, routeRequest(t, h, session, "POST", "/api/v1/rescan", strings.Repeat(" ", (1<<20)+1), nil), 413, &e)
}

func TestRoutesWrappedSentinelsAndInternalFailures(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{ErrInvalidRequest, 400, "invalid_request"}, {ErrStaleRevision, 409, "stale_revision"},
		{ErrNotFound, 404, "not_found"}, {ErrUnsafeFix, 422, "unsafe_fix"},
		{errors.New("private error"), 500, "internal_error"},
	} {
		s, _, _ := serviceFixture(t, 0, func(n int) error {
			if n > 1 {
				return fmt.Errorf("wrapped: %w", tc.err)
			}
			return nil
		})
		session := fixedSession(t)
		var e APIError
		routeJSON(t, routeRequest(t, NewHandler(s, session), session, "POST", "/api/v1/rescan", `{"revision":1}`, nil), tc.status, &e)
		if e.Code != tc.code {
			t.Fatalf("wrapped error: %+v", e)
		}
	}
	// Snapshot store failures and exporter validation failures must stay JSON,
	// never a partial success or a ZIP with error text appended.
	s, _, _ := serviceFixture(t, 0, nil)
	if err := os.MkdirAll(s.config.AppDataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.config.AppDataDir, "snapshots"), []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	session := fixedSession(t)
	var e APIError
	routeJSON(t, routeRequest(t, NewHandler(s, session), session, "GET", "/api/v1/snapshots", "", nil), 500, &e)
	w := routeRequest(t, newHandler(s, session, nil, "invalid/version"), session, "POST", "/api/v1/export", `{"revision":1}`, nil)
	routeJSON(t, w, 500, &e)
	if w.Header().Get("Content-Disposition") != "" {
		t.Fatal("failed export committed download headers")
	}
}

func TestRoutesExportUsesInjectedVersion(t *testing.T) {
	s, _, _ := serviceFixture(t, 0, nil)
	session := fixedSession(t)
	w := routeRequest(t, newHandler(s, session, nil, "0.2.0-test+abc"), session, "POST", "/api/v1/export", `{"revision":1}`, nil)
	if w.Code != 200 {
		t.Fatalf("export: %d %s", w.Code, w.Body.String())
	}
	archive, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range archive.File {
		if file.Name != "manifest.json" {
			continue
		}
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		defer reader.Close()
		var manifest struct {
			ToolVersion string `json:"tool_version"`
		}
		if err := json.NewDecoder(reader).Decode(&manifest); err != nil {
			t.Fatal(err)
		}
		if manifest.ToolVersion != "0.2.0-test+abc" {
			t.Fatalf("version discarded: %s", manifest.ToolVersion)
		}
		return
	}
	t.Fatal("manifest missing")
}

func TestRoutesReadResponsesRescrubCredentialsAndSessionToken(t *testing.T) {
	root := t.TempDir()
	session := fixedSession(t)
	credential := "sk-abcdefghijklmnopqrstuvwxyz123456"
	s, err := NewService(ServiceConfig{Workspace: root, HomeDir: filepath.Dir(root), AppDataDir: filepath.Join(root, "data"), Scan: func(context.Context) (model.ScanResult, error) {
		return model.ScanResult{SchemaVersion: model.ReportSchemaVersion, Analysis: model.Analysis{
			Clients: []model.ClientResult{{ID: "codex"}, {ID: "claude"}},
			Graph:   model.Graph{Nodes: []model.ConfigNode{{ID: "node-a", Type: model.NodeEffective, Client: "codex", DisplayName: credential + " " + session.Token()}}},
		}}, nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"/api/v1/state", "/api/v1/explain?id=node-a", "/api/v1/compare?left=codex&right=claude"} {
		w := routeRequest(t, NewHandler(s, session), session, "GET", target, "", nil)
		var result any
		routeJSON(t, w, 200, &result)
		if strings.Contains(w.Body.String(), credential) || strings.Contains(w.Body.String(), session.Token()) {
			t.Fatalf("credential leaked through %s", target)
		}
		if !strings.Contains(w.Body.String(), "[REDACTED]") {
			t.Fatal("fixture was not represented")
		}
	}
}

func TestRoutesRedactionCannotCrossJSONFieldBoundaries(t *testing.T) {
	root := t.TempDir()
	s, err := NewService(ServiceConfig{Workspace: root, HomeDir: filepath.Dir(root), AppDataDir: filepath.Join(root, "data"), Scan: func(context.Context) (model.ScanResult, error) {
		return model.ScanResult{Analysis: model.Analysis{Graph: model.Graph{Nodes: []model.ConfigNode{{ID: "node-a", DisplayName: "https://alice", LoadCondition: "password@example.test"}}}}}, nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	session := fixedSession(t)
	var detail ExplainResponse
	routeJSON(t, routeRequest(t, NewHandler(s, session), session, "GET", "/api/v1/explain?id=node-a", "", nil), 200, &detail)
	if detail.Node.DisplayName != "https://alice" || detail.Node.LoadCondition != "password@example.test" {
		t.Fatal("unrelated string fields were merged during redaction")
	}
	// Keys are scrubbed individually too, and large integer revisions retain precision.
	w := httptest.NewRecorder()
	writeJSON(w, 200, map[string]any{"sk-abcdefghijklmnopqrstuvwxyz": "secret-key", "revision": uint64(9007199254740993)}, "")
	var fields map[string]json.RawMessage
	routeJSON(t, w, 200, &fields)
	if _, ok := fields["[REDACTED]"]; !ok || string(fields["revision"]) != "9007199254740993" {
		t.Fatal("key redaction or integer precision lost")
	}
}

func TestRoutesExportRemovesSessionTokenBeforeHashing(t *testing.T) {
	root := t.TempDir()
	// An ordinary alphabetic 43-character base64url token also fits metadata IDs.
	session, err := NewSession(bytes.NewReader(bytes.Repeat([]byte{0x11}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	token := session.Token()
	s, err := NewService(ServiceConfig{Workspace: root, HomeDir: filepath.Dir(root), AppDataDir: filepath.Join(root, "data"), Scan: func(context.Context) (model.ScanResult, error) {
		return model.ScanResult{SchemaVersion: model.ReportSchemaVersion, Analysis: model.Analysis{
			Clients: []model.ClientResult{{ID: token, Compatibility: model.CompatibilityMetadata{Tier: model.TierVerified}}},
			Graph:   model.Graph{Nodes: []model.ConfigNode{{ID: "node-a", DisplayName: token, Attributes: map[string]model.SafeValue{token: {Display: "nested " + token}}}}},
		}}, nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	s.public.Drift = &snapshots.Diff{SchemaVersion: snapshots.SchemaVersion, Baseline: token, Changes: []snapshots.Change{{Kind: snapshots.ChangeAdded, EntityType: snapshots.EntityNode, ID: token, Summary: "node " + token + " added"}}}
	s.mu.Unlock()
	w := routeRequest(t, newHandler(s, session, nil, token), session, "POST", "/api/v1/export", fmt.Sprintf(`{"revision":1,"baseline":%q}`, token), nil)
	if w.Code != 200 {
		t.Fatalf("export status %d: %s", w.Code, w.Body.String())
	}
	archive, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	members := map[string][]byte{}
	for _, file := range archive.File {
		r, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
		members[file.Name] = data
		if bytes.Contains(data, []byte(token)) {
			t.Errorf("session token leaked in %s", file.Name)
		}
		if strings.HasSuffix(file.Name, ".json") && !json.Valid(data) {
			t.Errorf("invalid JSON member %s", file.Name)
		}
	}
	// Decode the offline report's embedded JSON, not only visible HTML text.
	html := string(members["report.html"])
	start := strings.Index(html, `data-encoding="base64">`)
	if start < 0 {
		t.Fatal("embedded report missing")
	}
	encoded := html[start+len(`data-encoding="base64">`):]
	encoded = strings.SplitN(encoded, "</script>", 2)[0]
	embedded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(embedded, []byte(token)) {
		t.Error("token leaked through embedded JSON")
	}
	var manifest struct {
		Members []struct{ Name, SHA256 string }
	}
	if err := json.Unmarshal(members["manifest.json"], &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Members) != 4 {
		t.Fatalf("missing manifest members: %+v", manifest)
	}
	for _, item := range manifest.Members {
		sum := sha256.Sum256(members[item.Name])
		if item.SHA256 != hex.EncodeToString(sum[:]) {
			t.Errorf("stale hash: %s", item.Name)
		}
	}
	if s.State().Revision != 1 || s.State().Drift.Baseline != token {
		t.Fatal("export mutated public state")
	}
}

func TestRoutesConcurrentFixTargetChangeReturnsConflict(t *testing.T) {
	var secondTarget string
	s, _, _ := serviceFixture(t, 2, func(n int) error {
		if n == 3 {
			return os.WriteFile(secondTarget, []byte("concurrent edit\n"), 0o640)
		}
		return nil
	})
	state := s.State()
	firstTarget := filepath.Join(s.config.Workspace, state.FixPlans[0].Edits[0].TargetPath)
	secondTarget = filepath.Join(s.config.Workspace, state.FixPlans[1].Edits[0].TargetPath)
	body, err := json.Marshal(ApplyRequest{Revision: 1, FixIDs: []string{state.FixPlans[0].ID, state.FixPlans[1].ID}})
	if err != nil {
		t.Fatal(err)
	}
	session := fixedSession(t)
	var apiErr APIError
	routeJSON(t, routeRequest(t, NewHandler(s, session), session, "POST", "/api/v1/fixes/apply", string(body), nil), 409, &apiErr)
	if apiErr.Code != "target_changed" || s.State().Revision != 1 {
		t.Fatalf("wrong conflict handling: %+v", apiErr)
	}
	first, _ := os.ReadFile(firstTarget)
	second, _ := os.ReadFile(secondTarget)
	if string(first) != "keep\nkeep\n" || string(second) != "concurrent edit\n" {
		t.Fatal("conflict failed to preserve rollback or concurrent edit")
	}
}
