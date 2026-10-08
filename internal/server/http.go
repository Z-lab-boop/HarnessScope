package server

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type HTTPConfig struct {
	Host        string
	Port        int
	Service     *Service
	Random      io.Reader
	Logger      *log.Logger
	ToolVersion string
}

//go:embed assets/*
var assets embed.FS

// Instance owns the listener and its shutdown. Wait completes only after the
// listener and the bounded graceful shutdown have both finished.
type Instance struct {
	server   *http.Server
	url      string
	stop     chan struct{}
	done     chan struct{}
	stopOnce sync.Once
	err      error // Published by closing done.
}

func Start(ctx context.Context, config HTTPConfig) (*Instance, error) {
	host := config.Host
	if host == "" {
		host = "127.0.0.1"
	}
	if err := ValidateListenHost(host); err != nil {
		return nil, err
	}
	if config.Port < 0 || config.Port > 65535 {
		return nil, fmt.Errorf("dashboard port must be between 0 and 65535")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if config.Service == nil || config.Service.State().Revision == 0 {
		return nil, fmt.Errorf("dashboard service must have an initial scan")
	}
	// The HTTP entry point deliberately binds IPv4 loopback only, even if the
	// admission policy also recognizes the IPv6 loopback spelling.
	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(config.Port)))
	if err != nil {
		return nil, err
	}
	session, err := NewSession(config.Random)
	if err != nil {
		listener.Close()
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		listener.Close()
		return nil, err
	}
	api := newHandler(config.Service, session, config.Logger, config.ToolVersion)
	requestCtx, cancelRequests := context.WithCancel(ctx)
	i := &Instance{url: "http://" + listener.Addr().String() + "/#token=" + session.Token(), stop: make(chan struct{}), done: make(chan struct{})}
	i.server = &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			setSecurityHeaders(w.Header())
			if strings.HasPrefix(r.URL.Path, "/api/v1/") {
				api.ServeHTTP(w, r)
				return
			}
			serveAsset(w, r)
		}),
		BaseContext:       func(net.Listener) context.Context { return requestCtx },
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second,
		WriteTimeout: 30 * time.Second, IdleTimeout: 30 * time.Second,
		ErrorLog: log.New(httpErrorWriter{&apiHandler{session: session, logger: config.Logger}}, "", 0),
	}
	served := make(chan error, 1)
	go func() { served <- i.server.Serve(listener) }()
	go func() {
		var serveErr error
		var serveFinished bool
		select {
		case serveErr = <-served:
			serveFinished = true
		case <-ctx.Done():
		case <-i.stop:
		}
		cancelRequests()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		shutdownErr := i.server.Shutdown(shutdownCtx)
		cancel()
		if shutdownErr != nil {
			_ = i.server.Close()
		}
		if !serveFinished {
			serveErr = <-served
		}
		if errors.Is(serveErr, http.ErrServerClosed) {
			serveErr = nil
		}
		i.err = errors.Join(serveErr, shutdownErr)
		close(i.done)
	}()
	return i, nil
}

func (i *Instance) URL() string { return i.url }
func (i *Instance) Wait() error { <-i.done; return i.err }
func (i *Instance) Close(ctx context.Context) error {
	i.stopOnce.Do(func() { close(i.stop) })
	select {
	case <-i.done:
		return i.err
	case <-ctx.Done():
		_ = i.server.Close()
		return ctx.Err()
	}
}

// Exact names avoid directory listings, redirects, SPA fallbacks, or exposing
// other embedded files. The launch credential is never interpolated into assets.
func serveAsset(w http.ResponseWriter, r *http.Request) {
	var name, contentType string
	switch r.URL.Path {
	case "/":
		name, contentType = "index.html", "text/html; charset=utf-8"
	case "/assets/dashboard.js":
		name, contentType = "dashboard.js", "text/javascript; charset=utf-8"
	case "/assets/dashboard.css":
		name, contentType = "dashboard.css", "text/css; charset=utf-8"
	default:
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	data, err := assets.ReadFile("assets/" + name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	if r.Method == http.MethodGet {
		_, _ = w.Write(data)
	}
}

// net/http diagnostics (including handler panic values) need the same token
// and path scrubbing as API errors; raw server logs must never bypass it.
type httpErrorWriter struct{ api *apiHandler }

func (w httpErrorWriter) Write(data []byte) (int, error) {
	w.api.logError(errors.New(string(data)))
	return len(data), nil
}
