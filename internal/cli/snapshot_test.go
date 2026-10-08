package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Z-lab-boop/harnessscope/internal/adapters"
	"github.com/Z-lab-boop/harnessscope/internal/model"
	"github.com/Z-lab-boop/harnessscope/internal/snapshots"
)

func runDiagnostic(t *testing.T, runtime *Runtime, want int, args ...string) string {
	t.Helper()
	var out, diagnostic bytes.Buffer
	code := ExecuteWithRuntime(context.Background(), args, &out, &diagnostic, runtime)
	if code != want {
		t.Fatalf("%v: code=%d want=%d stderr=%s", args, code, want, diagnostic.String())
	}
	return out.String()
}

func TestSnapshotCommandsSanitizeListAndCompareWithoutWorkspaceWrites(t *testing.T) {
	workspace := t.TempDir()
	runtime := fixtureRuntime(t, commandFixtureAdapter{id: "codex"})
	node := model.ConfigNode{ID: "node_one", Client: "codex", Type: model.NodeInstruction, DisplayName: filepath.Join(workspace, "private.md"), AdapterConfidence: model.EvidenceConfirmed}
	runtime.Registry = adapters.NewRegistry(commandFixtureAdapter{id: "codex", nodes: []model.ConfigNode{node}})
	saved := runDiagnostic(t, runtime, ExitOK, "snapshot", "save", "base", workspace)
	path := filepath.Join(runtime.Environment.AppDataDir, "snapshots", "base.json")
	if strings.TrimSpace(saved) != path {
		t.Fatalf("save stdout=%q", saved)
	}
	for path, mode := range map[string]os.FileMode{path: 0o600, filepath.Dir(path): 0o700} {
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != mode {
			t.Fatalf("mode %s: %v %v", path, info, err)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte(workspace)) || bytes.Contains(data, []byte(runtime.Environment.HomeDir)) {
		t.Fatal("snapshot leaked absolute roots")
	}
	var list []snapshots.Metadata
	if err := json.Unmarshal([]byte(runDiagnostic(t, runtime, ExitOK, "snapshot", "list")), &list); err != nil || len(list) != 1 || list[0].Name != "base" {
		t.Fatalf("list=%v err=%v", list, err)
	}
	var diff snapshots.Diff
	if err := json.Unmarshal([]byte(runDiagnostic(t, runtime, ExitOK, "snapshot", "diff", "base", workspace)), &diff); err != nil || len(diff.Changes) != 0 || diff.Baseline != "base" {
		t.Fatalf("diff=%+v err=%v", diff, err)
	}
	runtime.Registry = adapters.NewRegistry(commandFixtureAdapter{id: "codex"})
	if err := json.Unmarshal([]byte(runDiagnostic(t, runtime, ExitOK, "snapshot", "diff", "base", workspace)), &diff); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range diff.Changes {
		if c.ID == "node_one" && c.Kind == "REMOVED" && c.EntityType == "NODE" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing removal: %+v", diff)
	}
	runDiagnostic(t, runtime, ExitOperational, "snapshot", "save", "../escape", workspace)
	runDiagnostic(t, runtime, ExitOperational, "snapshot", "diff", "missing", workspace)
	if got := directoryNames(t, workspace); len(got) != 0 {
		t.Fatalf("workspace pollution: %v", got)
	}
}

func TestSnapshotRejectsRelativeOrEmptyAppData(t *testing.T) {
	for _, root := range []string{"", "relative-app-data"} {
		runtime := fixtureRuntime(t, commandFixtureAdapter{id: "codex"})
		runtime.Environment.AppDataDir = root
		runDiagnostic(t, runtime, ExitOperational, "snapshot", "list")
	}
}
