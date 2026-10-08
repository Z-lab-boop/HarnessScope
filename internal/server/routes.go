package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"strings"

	"github.com/Z-lab-boop/harnessscope/internal/fixes"
	"github.com/Z-lab-boop/harnessscope/internal/secrets"
)

// NewHandler exposes the authenticated local API. A real HTTP server supplies
// LocalAddrContextKey; synthetic callers must supply the listener address.
func NewHandler(service *Service, session Session) http.Handler {
	return newHandler(service, session, nil, "0.2.0")
}

// The listener layer injects its logger and actual build version here.
func newHandler(service *Service, session Session, logger *log.Logger, toolVersion string) http.Handler {
	a := &apiHandler{service: service, session: session, logger: logger, toolVersion: toolVersion}
	mux := http.NewServeMux()
	methods := make(map[string][]string)
	register := func(method, path string, handler http.HandlerFunc) {
		methods[path] = append(methods[path], method)
		mux.HandleFunc(method+" "+path, handler)
	}
	register("GET", "/api/v1/state", func(w http.ResponseWriter, r *http.Request) { a.result(w, service.State(), nil) })
	register("GET", "/api/v1/explain", func(w http.ResponseWriter, r *http.Request) {
		query, err := exactQuery(r, "id")
		if err != nil {
			a.result(w, nil, err)
			return
		}
		value, err := service.Explain(query.Get("id"))
		a.result(w, value, err)
	})
	register("GET", "/api/v1/compare", func(w http.ResponseWriter, r *http.Request) {
		query, err := exactQuery(r, "left", "right")
		if err != nil {
			a.result(w, nil, err)
			return
		}
		value, err := service.CompareClients(query.Get("left"), query.Get("right"))
		a.result(w, value, err)
	})
	register("GET", "/api/v1/backups", func(w http.ResponseWriter, r *http.Request) { a.result(w, service.State().Backups, nil) })
	register("GET", "/api/v1/snapshots", func(w http.ResponseWriter, r *http.Request) {
		value, err := service.ListSnapshots(r.Context())
		a.result(w, value, err)
	})
	register("POST", "/api/v1/rescan", mutation(a, func(r *http.Request, body RevisionRequest) (any, error) {
		return service.Rescan(r.Context(), body.Revision)
	}, 200))
	register("POST", "/api/v1/fixes/plan", mutation(a, func(r *http.Request, body RevisionRequest) (any, error) {
		return service.PlanFixes(r.Context(), body.Revision, nil)
	}, 200))
	register("POST", "/api/v1/fixes/apply", mutation(a, func(r *http.Request, body ApplyRequest) (any, error) {
		return service.ApplyFixes(r.Context(), body.Revision, body.FixIDs)
	}, 200))
	register("POST", "/api/v1/rollback", mutation(a, func(r *http.Request, body RollbackRequest) (any, error) {
		return service.Rollback(r.Context(), body.Revision, body.BackupID)
	}, 200))
	register("POST", "/api/v1/snapshots", mutation(a, func(r *http.Request, body SnapshotRequest) (any, error) {
		return service.SaveSnapshot(r.Context(), body.Revision, body.Name)
	}, 201))
	register("POST", "/api/v1/drift", mutation(a, func(r *http.Request, body DriftRequest) (any, error) {
		return service.CompareSnapshot(r.Context(), body.Revision, body.Baseline)
	}, 200))
	register("POST", "/api/v1/export", a.export)
	return session.Protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		allowed, exists := methods[r.URL.Path]
		if !exists {
			writeAPIError(w, 404, "not_found", "The requested resource was not found.")
			return
		}
		valid := false
		for _, method := range allowed {
			if r.Method == method {
				valid = true
			}
		}
		if !valid {
			w.Header().Set("Allow", strings.Join(allowed, ", "))
			writeAPIError(w, 405, "method_not_allowed", "The request method is not allowed.")
			return
		}
		if r.URL.Path != "/api/v1/explain" && r.URL.Path != "/api/v1/compare" && r.URL.RawQuery != "" {
			a.result(w, nil, ErrInvalidRequest)
			return
		}
		// Read routes do not accept hidden JSON/path parameters in a body.
		if r.Method == "GET" && r.Body != nil {
			body, err := io.ReadAll(r.Body)
			if err != nil || len(body) != 0 {
				a.result(w, nil, ErrInvalidRequest)
				return
			}
		}
		mux.ServeHTTP(w, r)
	}))
}

type apiHandler struct {
	service     *Service
	session     Session
	logger      *log.Logger
	toolVersion string
}

func mutation[T any](a *apiHandler, call func(*http.Request, T) (any, error), status int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body T
		if err := decodeStrict(w, r, &body); err != nil {
			a.result(w, nil, err)
			return
		}
		value, err := call(r, body)
		if err != nil {
			a.result(w, nil, err)
			return
		}
		writeJSON(w, status, value, a.session.Token())
	}
}

// decodeStrict requires one object with exact, unique field names and a
// revision. A second decode through the typed schema rejects unknown fields
// and wrong types; null never silently becomes a zero-value request.
func decodeStrict[T any](w http.ResponseWriter, r *http.Request, target *T) error {
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxSessionBody))
	if err != nil {
		var limit *http.MaxBytesError
		if errors.As(err, &limit) {
			return err
		}
		return ErrInvalidRequest
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	first, err := decoder.Token()
	if err != nil || first != json.Delim('{') {
		return ErrInvalidRequest
	}
	fields := map[string]bool{}
	typ := reflect.TypeOf(target).Elem()
	for i := 0; i < typ.NumField(); i++ {
		fields[strings.Split(typ.Field(i).Tag.Get("json"), ",")[0]] = true
	}
	seen := map[string]bool{}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return ErrInvalidRequest
		}
		key, ok := token.(string)
		if !ok || !fields[key] || seen[key] {
			return ErrInvalidRequest
		}
		seen[key] = true
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return ErrInvalidRequest
		}
	}
	if _, err := decoder.Token(); err != nil {
		return ErrInvalidRequest
	}
	if decoder.Decode(&struct{}{}) != io.EOF || !seen["revision"] {
		return ErrInvalidRequest
	}
	decoder = json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return ErrInvalidRequest
	}
	return nil
}

func exactQuery(r *http.Request, names ...string) (url.Values, error) {
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil || len(values) != len(names) {
		return nil, ErrInvalidRequest
	}
	for _, name := range names {
		if len(values[name]) != 1 || values.Get(name) == "" {
			return nil, ErrInvalidRequest
		}
	}
	return values, nil
}

func (a *apiHandler) export(w http.ResponseWriter, r *http.Request) {
	var body ExportRequest
	if err := decodeStrict(w, r, &body); err != nil {
		a.result(w, nil, err)
		return
	}
	// Complete validation before committing download headers or any ZIP bytes.
	var output bytes.Buffer
	if _, err := a.service.exportActive(r.Context(), body.Revision, &output, a.toolVersion, body.Baseline, []string{a.session.Token()}); err != nil {
		a.result(w, nil, err)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="harnessscope-diagnostics.zip"`)
	w.WriteHeader(http.StatusOK)
	if _, err := io.Copy(w, &output); err != nil {
		a.logError(err)
	}
}

func (a *apiHandler) result(w http.ResponseWriter, value any, err error) {
	if err == nil {
		writeJSON(w, http.StatusOK, value, a.session.Token())
		return
	}
	var limit *http.MaxBytesError
	switch {
	case errors.Is(err, ErrStaleRevision):
		writeAPIError(w, 409, "stale_revision", "The dashboard revision or active baseline changed. Refresh and retry.")
	case errors.Is(err, fixes.ErrTargetChanged):
		writeAPIError(w, 409, "target_changed", "A fix target changed. Rescan and retry.")
	case errors.Is(err, ErrInvalidRequest):
		writeAPIError(w, 400, "invalid_request", "The request is invalid.")
	case errors.Is(err, ErrNotFound):
		writeAPIError(w, 404, "not_found", "The requested item was not found.")
	case errors.Is(err, ErrUnsafeFix):
		writeAPIError(w, 422, "unsafe_fix", "Only SAFE fixes can be applied.")
	case errors.As(err, &limit):
		writeAPIError(w, 413, "body_too_large", "The request body is too large.")
	default:
		a.logError(err)
		writeAPIError(w, 500, "internal_error", "An internal error occurred.")
	}
}

var logPath = regexp.MustCompile(`[A-Za-z]:[\\/][^\s"'<>]+|/[^\s"'<>]+`)
var logCredential = regexp.MustCompile(`(?i)(token|password|secret|api_key|authorization)\s*[:=]\s*\S+`)

func (a *apiHandler) logError(err error) {
	if a.logger == nil {
		return
	}
	message := secrets.NewRedactor().ScrubText(err.Error())
	if a.session.Token() != "" {
		message = strings.ReplaceAll(message, a.session.Token(), "[REDACTED]")
	}
	message = logCredential.ReplaceAllString(message, "[REDACTED]")
	message = logPath.ReplaceAllString(message, "[PATH]")
	// Quote to keep untrusted line breaks from forging additional log entries.
	a.logger.Printf("dashboard API error: %q", message)
}

func writeAPIError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, APIError{Code: code, Message: secrets.NewRedactor().ScrubText(message), Details: map[string]string{}}, "")
}

func writeJSON(w http.ResponseWriter, status int, value any, sessionToken string) {
	redactor := secrets.NewRedactor()
	data, err := secrets.MapJSONStrings(value, func(text string) string {
		text = redactor.ScrubText(text)
		if sessionToken != "" {
			text = strings.ReplaceAll(text, sessionToken, "[REDACTED]")
		}
		return text
	})
	if err != nil {
		status = 500
		data = []byte(`{"code":"internal_error","message":"An internal error occurred.","details":{}}`)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(append(data, '\n'))
}
