package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func fixedSession(t *testing.T) Session {
	t.Helper()
	session, err := NewSession(bytes.NewReader(bytes.Repeat([]byte{0xfb}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	return session
}

func localRequest(method, host, body string) *http.Request {
	r := httptest.NewRequest(method, "http://"+host+"/api/v1/rescan", strings.NewReader(body))
	h, p, _ := net.SplitHostPort(host)
	a, _ := net.ResolveTCPAddr("tcp", net.JoinHostPort(h, p))
	return r.WithContext(context.WithValue(r.Context(), http.LocalAddrContextKey, a))
}

func assertSecurityHeaders(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()
	for name, want := range map[string]string{
		"Content-Security-Policy": "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'",
		"X-Content-Type-Options":  "nosniff",
		"Referrer-Policy":         "no-referrer",
		"Cache-Control":           "no-store",
		"Pragma":                  "no-cache",
		"Expires":                 "0",
	} {
		if got := response.Header().Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("CORS was enabled: %q", got)
	}
}

func TestSessionEntropyAndEncoding(t *testing.T) {
	entropy := bytes.Repeat([]byte{0xfb}, 33)
	random := bytes.NewReader(entropy)
	session, err := NewSession(random)
	if err != nil {
		t.Fatal(err)
	}
	if random.Len() != 1 {
		t.Fatalf("read %d bytes, want 32", len(entropy)-random.Len())
	}
	if len(session.Token()) != 43 || strings.ContainsAny(session.Token(), "+/=") {
		t.Fatalf("token is not unpadded base64url: %q", session.Token())
	}
	decoded, err := base64.RawURLEncoding.DecodeString(session.Token())
	if err != nil || !bytes.Equal(decoded, entropy[:32]) {
		t.Fatalf("encoded entropy mismatch: %x, %v", decoded, err)
	}
	for _, random := range []io.Reader{strings.NewReader("short"), failingEntropyReader{}} {
		session, err := NewSession(random)
		if err == nil || session.Token() != "" {
			t.Fatalf("entropy failure produced a usable session: %v", err)
		}
	}
	first, err := NewSession(nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewSession(nil)
	if err != nil || len(first.Token()) != 43 || first.Token() == second.Token() {
		t.Fatalf("default random sessions invalid: %v", err)
	}
}

type failingEntropyReader struct{}

func (failingEntropyReader) Read([]byte) (int, error) {
	return 0, errors.New("entropy unavailable")
}

func TestSessionSecurityMatrix(t *testing.T) {
	session := fixedSession(t)
	tests := []struct {
		name   string
		method string
		change func(*http.Request)
		want   int
	}{
		{"read without origin", http.MethodGet, nil, http.StatusNoContent},
		{"head without origin", http.MethodHead, nil, http.StatusNoContent},
		{"same origin mutation", http.MethodPost, nil, http.StatusNoContent},
		{"missing token", http.MethodGet, func(r *http.Request) { r.Header.Del("X-HarnessScope-Token") }, http.StatusUnauthorized},
		{"wrong token", http.MethodGet, func(r *http.Request) { r.Header.Set("X-HarnessScope-Token", strings.Repeat("a", 43)) }, http.StatusUnauthorized},
		{"short token", http.MethodGet, func(r *http.Request) { r.Header.Set("X-HarnessScope-Token", "a") }, http.StatusUnauthorized},
		{"duplicate token", http.MethodGet, func(r *http.Request) { r.Header.Add("X-HarnessScope-Token", session.Token()) }, http.StatusUnauthorized},
		{"remote host", http.MethodGet, func(r *http.Request) { r.Host = "evil.example:8123" }, http.StatusForbidden},
		{"remote ip", http.MethodGet, func(r *http.Request) { r.Host = "192.0.2.1:8123" }, http.StatusForbidden},
		{"localhost name", http.MethodGet, func(r *http.Request) { r.Host = "localhost:8123" }, http.StatusForbidden},
		{"missing port", http.MethodGet, func(r *http.Request) { r.Host = "127.0.0.1" }, http.StatusForbidden},
		{"wrong active port", http.MethodGet, func(r *http.Request) { r.Host = "127.0.0.1:8124" }, http.StatusForbidden},
		{"zero port", http.MethodGet, func(r *http.Request) { r.Host = "127.0.0.1:0" }, http.StatusForbidden},
		{"padded port", http.MethodGet, func(r *http.Request) { r.Host = "127.0.0.1:08123" }, http.StatusForbidden},
		{"different loopback", http.MethodGet, func(r *http.Request) { r.Host = "127.0.0.2:8123" }, http.StatusForbidden},
		{"host suffix", http.MethodGet, func(r *http.Request) { r.Host = "127.0.0.1:8123.evil.example" }, http.StatusForbidden},
		{"no local address", http.MethodGet, func(r *http.Request) { *r = *r.WithContext(context.Background()) }, http.StatusForbidden},
		{"remote listener", http.MethodGet, func(r *http.Request) {
			*r = *r.WithContext(context.WithValue(r.Context(), http.LocalAddrContextKey, &net.TCPAddr{IP: net.ParseIP("192.0.2.1"), Port: 8123}))
		}, http.StatusForbidden},
		{"missing mutation origin", http.MethodPost, func(r *http.Request) { r.Header.Del("Origin") }, http.StatusForbidden},
		{"foreign mutation origin", http.MethodPost, func(r *http.Request) { r.Header.Set("Origin", "http://evil.example:8123") }, http.StatusForbidden},
		{"https origin", http.MethodPost, func(r *http.Request) { r.Header.Set("Origin", "https://127.0.0.1:8123") }, http.StatusForbidden},
		{"null origin", http.MethodPost, func(r *http.Request) { r.Header.Set("Origin", "null") }, http.StatusForbidden},
		{"origin path", http.MethodPost, func(r *http.Request) { r.Header.Set("Origin", "http://127.0.0.1:8123/") }, http.StatusForbidden},
		{"origin credentials", http.MethodPost, func(r *http.Request) { r.Header.Set("Origin", "http://user@127.0.0.1:8123") }, http.StatusForbidden},
		{"origin query", http.MethodPost, func(r *http.Request) { r.Header.Set("Origin", "http://127.0.0.1:8123?x") }, http.StatusForbidden},
		{"origin fragment", http.MethodPost, func(r *http.Request) { r.Header.Set("Origin", "http://127.0.0.1:8123#x") }, http.StatusForbidden},
		{"duplicate origin", http.MethodPost, func(r *http.Request) { r.Header.Add("Origin", "http://127.0.0.1:8123") }, http.StatusForbidden},
		{"foreign read origin", http.MethodGet, func(r *http.Request) { r.Header.Set("Origin", "http://evil.example") }, http.StatusForbidden},
		{"same read origin", http.MethodGet, func(r *http.Request) { r.Header.Set("Origin", "http://127.0.0.1:8123") }, http.StatusNoContent},
		{"missing content type", http.MethodPost, func(r *http.Request) { r.Header.Del("Content-Type") }, http.StatusUnsupportedMediaType},
		{"plain text", http.MethodPost, func(r *http.Request) { r.Header.Set("Content-Type", "text/plain") }, http.StatusUnsupportedMediaType},
		{"json with charset", http.MethodPost, func(r *http.Request) { r.Header.Set("Content-Type", "application/json; charset=utf-8") }, http.StatusNoContent},
		{"malformed parameter", http.MethodPost, func(r *http.Request) { r.Header.Set("Content-Type", "application/json; charset") }, http.StatusUnsupportedMediaType},
		{"duplicate content type", http.MethodPost, func(r *http.Request) { r.Header.Add("Content-Type", "text/plain") }, http.StatusUnsupportedMediaType},
		{"put missing origin", http.MethodPut, func(r *http.Request) { r.Header.Del("Origin") }, http.StatusForbidden},
		{"patch missing origin", http.MethodPatch, func(r *http.Request) { r.Header.Del("Origin") }, http.StatusForbidden},
		{"delete missing origin", http.MethodDelete, func(r *http.Request) { r.Header.Del("Origin") }, http.StatusForbidden},
		{"preflight missing origin", http.MethodOptions, func(r *http.Request) { r.Header.Del("Origin") }, http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := localRequest(tt.method, "127.0.0.1:8123", `{}`)
			r.Header.Set("X-HarnessScope-Token", session.Token())
			if tt.method != http.MethodGet && tt.method != http.MethodHead {
				r.Header.Set("Origin", "http://127.0.0.1:8123")
				r.Header.Set("Content-Type", "application/json")
			}
			if tt.change != nil {
				tt.change(r)
			}
			called := false
			response := httptest.NewRecorder()
			session.Protect(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusNoContent)
			})).ServeHTTP(response, r)
			if response.Code != tt.want || called != (tt.want == http.StatusNoContent) {
				t.Fatalf("code=%d called=%v, want code=%d", response.Code, called, tt.want)
			}
			assertSecurityHeaders(t, response)
			if strings.Contains(response.Body.String(), session.Token()) {
				t.Fatal("token leaked in error")
			}
		})
	}
}

func TestSessionIPv6(t *testing.T) {
	session := fixedSession(t)
	for _, host := range []string{"[::1]:8123", "[::1]:8124", "[::1%lo0]:8123", "::1:8123", "[0:0:0:0:0:0:0:1]:8123"} {
		t.Run(host, func(t *testing.T) {
			r := localRequest(http.MethodPost, "[::1]:8123", `{}`)
			r.Host = host
			r.Header.Set("X-HarnessScope-Token", session.Token())
			r.Header.Set("Origin", "http://"+host)
			r.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			session.Protect(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })).ServeHTTP(response, r)
			want := http.StatusForbidden
			if host == "[::1]:8123" {
				want = http.StatusNoContent
			}
			if response.Code != want {
				t.Fatalf("code=%d, want %d", response.Code, want)
			}
			assertSecurityHeaders(t, response)
		})
	}
}

func TestSessionBodyLimit(t *testing.T) {
	session := fixedSession(t)
	for _, unknown := range []bool{false, true} {
		for _, size := range []int{1 << 20, (1 << 20) + 1} {
			r := localRequest(http.MethodPost, "127.0.0.1:8123", strings.Repeat("x", size))
			if unknown {
				r.ContentLength = -1
			}
			r.Header.Set("X-HarnessScope-Token", session.Token())
			r.Header.Set("Origin", "http://127.0.0.1:8123")
			r.Header.Set("Content-Type", "application/json")
			called := false
			response := httptest.NewRecorder()
			session.Protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				body, err := io.ReadAll(r.Body)
				if err != nil || len(body) != size {
					t.Errorf("body size=%d, err=%v", len(body), err)
				}
				w.WriteHeader(http.StatusNoContent)
			})).ServeHTTP(response, r)
			want := http.StatusNoContent
			if size > 1<<20 {
				want = http.StatusRequestEntityTooLarge
			}
			if response.Code != want || called != (want == http.StatusNoContent) {
				t.Fatalf("unknown=%v size=%d code=%d called=%v", unknown, size, response.Code, called)
			}
			assertSecurityHeaders(t, response)
		}
	}
}

func TestSessionZeroValueRejectsEmptyToken(t *testing.T) {
	r := localRequest(http.MethodGet, "127.0.0.1:8123", "")
	response := httptest.NewRecorder()
	Session{}.Protect(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("zero session accepted") })).ServeHTTP(response, r)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("code=%d", response.Code)
	}
	assertSecurityHeaders(t, response)
}

func TestSessionBodyReadFailure(t *testing.T) {
	session := fixedSession(t)
	r := localRequest(http.MethodPost, "127.0.0.1:8123", "")
	r.Body = io.NopCloser(failingEntropyReader{})
	r.ContentLength = -1
	r.Header.Set("X-HarnessScope-Token", session.Token())
	r.Header.Set("Origin", "http://127.0.0.1:8123")
	r.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	session.Protect(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("invalid body reached handler") })).ServeHTTP(response, r)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("code=%d", response.Code)
	}
	assertSecurityHeaders(t, response)
	if strings.Contains(response.Body.String(), "entropy unavailable") {
		t.Fatal("internal reader error leaked")
	}
}

func TestSessionConcurrentRequests(t *testing.T) {
	session := fixedSession(t)
	handler := session.Protect(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	for i := 0; i < 16; i++ {
		t.Run("request", func(t *testing.T) {
			t.Parallel()
			r := localRequest(http.MethodGet, "127.0.0.1:8123", "")
			r.Header.Set("X-HarnessScope-Token", session.Token())
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, r)
			if response.Code != http.StatusNoContent {
				t.Fatalf("code=%d", response.Code)
			}
			assertSecurityHeaders(t, response)
		})
	}
}

func TestSessionRealListener(t *testing.T) {
	session := fixedSession(t)
	server := httptest.NewServer(session.Protect(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })))
	defer server.Close()
	r, err := http.NewRequest(http.MethodGet, server.URL+"/api/v1/state", nil)
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("X-HarnessScope-Token", session.Token())
	response, err := server.Client().Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("code=%d", response.StatusCode)
	}
}

func TestValidateListenHost(t *testing.T) {
	for _, host := range []string{"127.0.0.1", "::1", "", "localhost", "0.0.0.0", "::", "192.0.2.1", "127.0.0.2", "[::1]", "::1%lo0", "127.0.0.1:8123", " 127.0.0.1"} {
		wantOK := host == "127.0.0.1" || host == "::1"
		if err := ValidateListenHost(host); (err == nil) != wantOK {
			t.Errorf("host=%q err=%v", host, err)
		}
	}
}
