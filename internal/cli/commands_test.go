package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Z-lab-boop/harnessscope/internal/adapters"
	"github.com/Z-lab-boop/harnessscope/internal/discovery"
	"github.com/Z-lab-boop/harnessscope/internal/model"
)

type commandFixtureAdapter struct {
	id      string
	nodes   []model.ConfigNode
	finding *model.Finding
}

func (a commandFixtureAdapter) ID() string { return a.id }
func (a commandFixtureAdapter) Detect(context.Context) model.DetectionResult {
	return model.DetectionResult{Installed: true, Version: "fixture"}
}
func (a commandFixtureAdapter) DiscoverSources(context.Context, string) []model.ConfigSource {
	return []model.ConfigSource{{ID: "source_" + a.id, Client: a.id, LogicalPath: "/fixture/" + a.id + ".json", Exists: true, Readable: true}}
}
func (a commandFixtureAdapter) Parse(context.Context, model.ConfigSource) model.ParsedConfig {
	result := model.ParsedConfig{Client: a.id, Nodes: a.nodes}
	if a.finding != nil {
		result.Findings = []model.Finding{*a.finding}
	}
	return result
}
func (a commandFixtureAdapter) Resolve(_ context.Context, parsed []model.ParsedConfig) model.EffectiveConfig {
	result := model.EffectiveConfig{Client: a.id}
	for _, item := range parsed {
		result.Nodes = append(result.Nodes, item.Nodes...)
		result.Findings = append(result.Findings, item.Findings...)
	}
	return result
}
func (a commandFixtureAdapter) Capabilities() model.AdapterCapabilities {
	return model.AdapterCapabilities{Precedence: true}
}
func (a commandFixtureAdapter) Compatibility() model.CompatibilityMetadata {
	return model.CompatibilityMetadata{Tier: model.TierVerified, State: model.CompatibilityVerified, RulesetVersion: "fixture-v1"}
}

func TestScanNoReportDoesNotWriteWorkspace(t *testing.T) {
	workspace := t.TempDir()
	runtime := fixtureRuntime(t, commandFixtureAdapter{id: "codex"})
	before := directoryNames(t, workspace)
	var stdout, stderr bytes.Buffer

	code := ExecuteWithRuntime(context.Background(), []string{"scan", workspace, "--client", "codex", "--no-report"}, &stdout, &stderr, runtime)

	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	after := directoryNames(t, workspace)
	if strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Fatalf("scan wrote workspace: before=%v after=%v", before, after)
	}
	if !strings.Contains(stdout.String(), "codex") {
		t.Fatalf("terminal summary missing client: %s", stdout.String())
	}
}

func TestScanFailOnHighReturnsThresholdExitCode(t *testing.T) {
	high := model.Finding{RuleID: "PATH-0001", Severity: model.SeverityHigh, Evidence: model.EvidenceConfirmed, Summary: "broken", Reason: "missing", Impact: "cannot start", Remediation: "restore"}
	runtime := fixtureRuntime(t, commandFixtureAdapter{id: "codex", finding: &high})
	var stdout, stderr bytes.Buffer

	code := ExecuteWithRuntime(context.Background(), []string{"scan", t.TempDir(), "--client", "codex", "--no-report", "--fail-on", "high"}, &stdout, &stderr, runtime)

	if code != ExitThreshold {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
}

func TestExplainPrintsOriginChain(t *testing.T) {
	node := model.ConfigNode{
		ID: "node_playwright", Client: "codex", Type: model.NodeMCPServer, DisplayName: "mcp.playwright", AdapterConfidence: model.EvidenceConfirmed,
		Origins: []model.Origin{{SourceID: "source_codex", LogicalPath: "/fixture/config.toml", FieldPath: "mcp_servers.playwright", Scope: model.ScopeUser, Rule: "fixture-v1"}},
	}
	runtime := fixtureRuntime(t, commandFixtureAdapter{id: "codex", nodes: []model.ConfigNode{node}})
	var stdout, stderr bytes.Buffer

	code := ExecuteWithRuntime(context.Background(), []string{"explain", "mcp.playwright", "--client", "codex", "--path", t.TempDir()}, &stdout, &stderr, runtime)

	if code != 0 || !strings.Contains(stdout.String(), "/fixture/config.toml") || !strings.Contains(stdout.String(), "mcp_servers.playwright") {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
}

func TestCompareReportsMissingNormalizedElement(t *testing.T) {
	codexNode := model.ConfigNode{ID: "codex_rule", Client: "codex", Type: model.NodeRule, DisplayName: "setting.model", AdapterConfidence: model.EvidenceConfirmed}
	runtime := fixtureRuntime(t,
		commandFixtureAdapter{id: "codex", nodes: []model.ConfigNode{codexNode}},
		commandFixtureAdapter{id: "claude"},
	)
	var stdout, stderr bytes.Buffer

	code := ExecuteWithRuntime(context.Background(), []string{"compare", "codex", "claude", "--path", t.TempDir()}, &stdout, &stderr, runtime)

	if code != 0 || !strings.Contains(stdout.String(), "setting.model") || !strings.Contains(stdout.String(), "missing") {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
}

func fixtureRuntime(t *testing.T, clients ...adapters.ClientAdapter) *Runtime {
	t.Helper()
	env := discovery.NewEnvironment("darwin", t.TempDir(), nil)
	return &Runtime{Registry: adapters.NewRegistry(clients...), Environment: env}
}

func directoryNames(t *testing.T, path string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Clean(path))
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}
