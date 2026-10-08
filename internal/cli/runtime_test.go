package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Z-lab-boop/harnessscope/internal/model"
)

func TestAdvancedRuntimePassesCapturedEnvironmentAndAbsoluteRoot(t *testing.T) {
	root := t.TempDir()
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "available-tool"), []byte("fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	nodes := []model.ConfigNode{
		{ID: "resolved", Client: "codex", Type: model.NodeMCPServer, DisplayName: "mcp.resolved", AdapterConfidence: model.EvidenceConfirmed, Attributes: map[string]model.SafeValue{"command": {Display: "available-tool", Present: true}}},
		{ID: "missing", Client: "codex", Type: model.NodeHook, DisplayName: "hook.event", AdapterConfidence: model.EvidenceConfirmed, Attributes: map[string]model.SafeValue{"command": {Display: "missing-tool", Present: true}}},
		{ID: "directory", Client: "codex", Type: model.NodeMCPServer, DisplayName: "mcp.directory", AdapterConfidence: model.EvidenceConfirmed, Attributes: map[string]model.SafeValue{"cwd": {Display: root, Present: true}}},
	}
	runtime := fixtureRuntime(t, commandFixtureAdapter{id: "codex", nodes: nodes})
	runtime.Environment.PathEntries = []string{bin}
	relative, err := filepath.Rel(runtime.Environment.HomeDir, root)
	if err != nil {
		t.Fatal(err)
	}
	// Give Scan a relative path while keeping cwd changes local to this non-parallel test.
	t.Chdir(runtime.Environment.HomeDir)
	result, err := runtime.Scan(context.Background(), ScanOptions{CWD: relative})
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, f := range result.Analysis.Findings {
		counts[f.RuleID]++
	}
	if counts["CMD-0001"] != 1 || counts["PATH-0002"] != 1 {
		t.Fatalf("advanced options not wired: %v", counts)
	}
	data, err := json.Marshal(result.RunMetadata)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), bin) || strings.Contains(string(data), "path_entries") {
		t.Fatal("PATH entries published as run metadata")
	}
}
