package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Z-lab-boop/harnessscope/internal/model"
	"github.com/Z-lab-boop/harnessscope/internal/server"
)

const serveTestURL = "http://127.0.0.1:12345/#token=launch-test-credential"

type fakeServerInstance struct {
	wait         func() error
	closed       int
	boundedClose bool
}

func (*fakeServerInstance) URL() string { return serveTestURL }
func (f *fakeServerInstance) Wait() error {
	if f.wait != nil {
		return f.wait()
	}
	return nil
}
func (f *fakeServerInstance) Close(ctx context.Context) error {
	f.closed++
	deadline, ok := ctx.Deadline()
	f.boundedClose = ok && time.Until(deadline) <= 5*time.Second && ctx.Err() == nil
	return nil
}

type servePathAdapter struct {
	commandFixtureAdapter
	roots *[]string
}

func (a servePathAdapter) DiscoverSources(_ context.Context, root string) []model.ConfigSource {
	*a.roots = append(*a.roots, root)
	return nil
}

func TestServeFreezesWorkspaceClientsAndStorageAtStartup(t *testing.T) {
	initial := t.TempDir()
	workspace := filepath.Join(initial, "workspace")
	if err := os.Mkdir(workspace, 0o700); err != nil {
		t.Fatal(err)
	}
	other := t.TempDir()
	t.Chdir(initial)
	var roots []string
	runtime := fixtureRuntime(t, servePathAdapter{commandFixtureAdapter: commandFixtureAdapter{id: "codex"}, roots: &roots}, commandFixtureAdapter{id: "cursor"})
	runtime.Environment.AppDataDir = "relative-data"
	runtime.ToolVersion = "cli-test-build"
	instance := &fakeServerInstance{}
	opened := ""
	runtime.OpenBrowser = func(_ context.Context, target string) error { opened = target; return nil }
	runtime.StartServer = func(ctx context.Context, config server.HTTPConfig) (ServerInstance, error) {
		if config.Port != 8123 || config.Logger == nil || config.ToolVersion != "cli-test-build" {
			t.Fatalf("wrong HTTP config: port=%d version=%s logger=%v", config.Port, config.ToolVersion, config.Logger)
		}
		state := config.Service.State()
		if state.Revision != 1 || len(state.Result.Analysis.Clients) != 1 || state.Result.Analysis.Clients[0].ID != "codex" {
			t.Fatalf("initial scan not ready or wrong clients: %+v", state)
		}
		if err := os.Chdir(other); err != nil {
			t.Fatal(err)
		}
		if _, err := config.Service.Rescan(ctx, 1); err != nil {
			t.Fatal(err)
		}
		if _, err := config.Service.SaveSnapshot(ctx, 2, "startup-root"); err != nil {
			t.Fatal(err)
		}
		return instance, nil
	}
	var stdout, stderr bytes.Buffer
	code := ExecuteWithRuntime(context.Background(), []string{"serve", "workspace", "--client", "codex", "--port", "8123", "--open"}, &stdout, &stderr, runtime)
	if code != ExitOK {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if stdout.String() != serveTestURL+"\n" || opened != serveTestURL || stderr.Len() != 0 {
		t.Fatal("launch URL was not emitted exactly once or browser not opened")
	}
	if len(roots) != 2 || roots[0] != workspace || roots[1] != workspace {
		t.Fatalf("scan workspace moved: %v", roots)
	}
	if instance.closed != 1 || !instance.boundedClose {
		t.Fatal("missing bounded instance cleanup")
	}
	if entries, err := os.ReadDir(filepath.Join(initial, "relative-data", "snapshots")); err != nil || len(entries) != 1 {
		t.Fatalf("snapshot root moved: entries=%v err=%v", entries, err)
	}
	for _, path := range []string{filepath.Join(initial, "relative-data", "reports"), filepath.Join(other, "relative-data")} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("unexpected output at %s: %v", path, err)
		}
	}
	if names := directoryNames(t, workspace); len(names) != 0 {
		t.Fatalf("serve wrote workspace: %v", names)
	}
}

func TestServeWithoutOpenWaitsForContextCancellation(t *testing.T) {
	runtime := fixtureRuntime(t, commandFixtureAdapter{id: "codex"})
	workspace := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	opened := false
	runtime.OpenBrowser = func(context.Context, string) error { opened = true; return nil }
	instance := &fakeServerInstance{wait: func() error { cancel(); <-ctx.Done(); return nil }}
	runtime.StartServer = func(startCtx context.Context, config server.HTTPConfig) (ServerInstance, error) {
		if config.Port != 0 || config.Service.State().Revision != 1 {
			t.Fatal("default port or readiness incorrect")
		}
		instance.wait = func() error { cancel(); <-startCtx.Done(); return nil }
		return instance, nil
	}
	var stdout, stderr bytes.Buffer
	if code := ExecuteWithRuntime(ctx, []string{"serve", workspace}, &stdout, &stderr, runtime); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if opened || instance.closed != 1 || !instance.boundedClose || stdout.String() != serveTestURL+"\n" {
		t.Fatal("incorrect cancel/open/cleanup behavior")
	}
	if _, err := os.Stat(runtime.Environment.AppDataDir); !os.IsNotExist(err) {
		t.Fatalf("serve created app data: %v", err)
	}
}

func TestServeOperationalFailuresCloseInstanceWithoutRepeatingToken(t *testing.T) {
	for _, mode := range []string{"start", "wait", "browser", "output"} {
		t.Run(mode, func(t *testing.T) {
			runtime := fixtureRuntime(t, commandFixtureAdapter{id: "codex"})
			instance := &fakeServerInstance{}
			runtime.StartServer = func(context.Context, server.HTTPConfig) (ServerInstance, error) {
				if mode == "start" {
					return nil, errors.New("listen failed")
				}
				return instance, nil
			}
			runtime.OpenBrowser = func(context.Context, string) error { return errors.New("open failed " + serveTestURL) }
			if mode == "wait" {
				instance.wait = func() error { return errors.New("serve failed") }
			}
			var stdout, stderr bytes.Buffer
			var output io.Writer = &stdout
			if mode == "output" {
				output = brokenServeOutput{}
			}
			args := []string{"serve", t.TempDir()}
			if mode == "browser" {
				args = append(args, "--open")
			}
			code := ExecuteWithRuntime(context.Background(), args, output, &stderr, runtime)
			if code != ExitOperational {
				t.Fatalf("code=%d stderr=%s", code, stderr.String())
			}
			if mode != "start" && (instance.closed != 1 || !instance.boundedClose) {
				t.Fatal("failure leaked server")
			}
			if mode == "start" && stdout.Len() != 0 {
				t.Fatal("printed URL before successful startup")
			}
			if strings.Contains(stderr.String(), "launch-test-credential") {
				t.Fatal("token repeated in error output")
			}
		})
	}
}

type brokenServeOutput struct{}

func (brokenServeOutput) Write([]byte) (int, error) { return 0, errors.New("output failed") }

func TestServeInvalidInputNeverStartsListener(t *testing.T) {
	for _, args := range [][]string{{"--client", "unknown"}, {"--port", "-1"}, {"--port", "65536"}, {"--host", "0.0.0.0"}, {"one", "two"}} {
		runtime := fixtureRuntime(t, commandFixtureAdapter{id: "codex"})
		started := false
		runtime.StartServer = func(context.Context, server.HTTPConfig) (ServerInstance, error) {
			started = true
			return &fakeServerInstance{}, nil
		}
		var stdout, stderr bytes.Buffer
		if code := ExecuteWithRuntime(context.Background(), append([]string{"serve"}, args...), &stdout, &stderr, runtime); code != ExitOperational || started {
			t.Fatalf("accepted invalid args %v: code=%d started=%v", args, code, started)
		}
	}
}

type serveBlockingAdapter struct {
	commandFixtureAdapter
	entered chan struct{}
	release chan struct{}
}

func (a serveBlockingAdapter) Detect(ctx context.Context) model.DetectionResult {
	close(a.entered)
	select {
	case <-a.release:
	case <-ctx.Done():
	}
	return model.DetectionResult{Installed: true}
}

func TestServeWaitsForInitialScanAndCancelsStartup(t *testing.T) {
	for _, cancelStartup := range []bool{false, true} {
		t.Run(map[bool]string{false: "ready", true: "canceled"}[cancelStartup], func(t *testing.T) {
			entered, release, started := make(chan struct{}), make(chan struct{}), make(chan struct{})
			runtime := fixtureRuntime(t, serveBlockingAdapter{commandFixtureAdapter: commandFixtureAdapter{id: "codex"}, entered: entered, release: release})
			runtime.StartServer = func(context.Context, server.HTTPConfig) (ServerInstance, error) {
				close(started)
				return &fakeServerInstance{}, nil
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			workspace := t.TempDir()
			var stdout, stderr bytes.Buffer
			done := make(chan int, 1)
			go func() { done <- ExecuteWithRuntime(ctx, []string{"serve", workspace}, &stdout, &stderr, runtime) }()
			select {
			case <-entered:
			case <-time.After(3 * time.Second):
				t.Fatal("initial scan did not begin")
			}
			select {
			case <-started:
				t.Fatal("listener started before initial state")
			default:
			}
			if cancelStartup {
				cancel()
			} else {
				close(release)
			}
			select {
			case code := <-done:
				if cancelStartup {
					if code != ExitOperational || stdout.Len() != 0 {
						t.Fatalf("canceled startup code=%d output=%q", code, stdout.String())
					}
					select {
					case <-started:
						t.Fatal("canceled startup opened listener")
					default:
					}
				} else if code != ExitOK {
					t.Fatalf("ready startup code=%d stderr=%s", code, stderr.String())
				}
			case <-time.After(3 * time.Second):
				t.Fatal("startup did not finish")
			}
		})
	}
}
