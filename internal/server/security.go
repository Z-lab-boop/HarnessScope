package server

import (
	"bytes"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"strconv"
)

const maxSessionBody = 1 << 20

// Session is an immutable, process-local credential for dashboard API requests.
type Session struct {
	token string
}

// NewSession reads exactly 32 bytes of entropy. A nil reader uses crypto/rand.
func NewSession(random io.Reader) (Session, error) {
	if random == nil {
		random = rand.Reader
	}
	var entropy [32]byte
	if _, err := io.ReadFull(random, entropy[:]); err != nil {
		return Session{}, fmt.Errorf("generate dashboard session: %w", err)
	}
	return Session{token: base64.RawURLEncoding.EncodeToString(entropy[:])}, nil
}

// Token is used only for the one-time launch URL and authenticated API headers.
func (s Session) Token() string { return s.token }

// ValidateListenHost refuses wildcard addresses, names, and remote interfaces.
func ValidateListenHost(host string) error {
	if host != "127.0.0.1" && host != "::1" {
		return fmt.Errorf("dashboard must listen on 127.0.0.1 or ::1")
	}
	return nil
}

// Protect authenticates API requests against the actual connection endpoint.
// net/http supplies LocalAddrContextKey when serving a listener; callers using
// synthetic requests must supply it too. Missing listener information fails
// closed, so neither the Host header nor request URL can choose the endpoint.
func (s Session) Protect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setSecurityHeaders(w.Header())
		tokens := r.Header.Values("X-HarnessScope-Token")
		if len(tokens) != 1 || s.token == "" || subtle.ConstantTimeCompare([]byte(tokens[0]), []byte(s.token)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if !validSessionHost(r) {
			http.Error(w, "invalid dashboard host", http.StatusForbidden)
			return
		}
		mutation := r.Method != http.MethodGet && r.Method != http.MethodHead
		origins := r.Header.Values("Origin")
		// An Origin is the serialized scheme and authority only. Comparing that
		// exact serialization rejects paths, credentials, queries, fragments,
		// duplicate headers, and alternate schemes without normalizing them.
		if len(origins) > 1 || (len(origins) == 1 && origins[0] != "http://"+r.Host) || (mutation && len(origins) == 0) {
			http.Error(w, "invalid dashboard origin", http.StatusForbidden)
			return
		}
		if mutation {
			types := r.Header.Values("Content-Type")
			if len(types) != 1 {
				http.Error(w, "application/json is required", http.StatusUnsupportedMediaType)
				return
			}
			mediaType, _, err := mime.ParseMediaType(types[0])
			if err != nil || mediaType != "application/json" {
				http.Error(w, "application/json is required", http.StatusUnsupportedMediaType)
				return
			}
		}
		if r.ContentLength > maxSessionBody {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		if r.Body != nil {
			// Validate before invoking a handler, including chunked/unknown-size
			// requests. MaxBytesReader alone could let a handler mutate state
			// before discovering the over-limit suffix or ignore its read error.
			body, err := io.ReadAll(io.LimitReader(r.Body, maxSessionBody+1))
			r.Body.Close()
			if len(body) > maxSessionBody {
				http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
				return
			}
			if err != nil {
				http.Error(w, "invalid request body", http.StatusBadRequest)
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
		}
		next.ServeHTTP(w, r)
	})
}

func validSessionHost(r *http.Request) bool {
	host, port, err := net.SplitHostPort(r.Host)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return false
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 || strconv.Itoa(portNumber) != port {
		return false
	}
	if r.Host != net.JoinHostPort(ip.String(), port) {
		return false
	}
	local, ok := r.Context().Value(http.LocalAddrContextKey).(net.Addr)
	if !ok || local == nil {
		return false
	}
	return r.Host == local.String()
}

func setSecurityHeaders(headers http.Header) {
	headers.Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
	headers.Set("X-Content-Type-Options", "nosniff")
	headers.Set("Referrer-Policy", "no-referrer")
	headers.Set("Cache-Control", "no-store")
	headers.Set("Pragma", "no-cache")
	headers.Set("Expires", "0")
}
