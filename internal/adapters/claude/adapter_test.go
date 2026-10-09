package claude

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Z-lab-boop/harnessscope/internal/discovery"
	"github.com/Z-lab-boop/harnessscope/internal/model"
	"github.com/Z-lab-boop/harnessscope/internal/secrets"
)

func TestVerifiedFixtureResolvesLocalSettingsAndImports(t *testing.T) {
	fixture := fixturePath(t, "basic")
	env := discovery.NewEnvironment("darwin", filepath.Join(fixture, "home"), []string{filepath.Join(fixture, "bin")})
	adapter := New(env, secrets.NewRedactor(), WithManagedSettingsPath(filepath.Join(fixture, "managed-settings.json")))
	ctx := context.Background()

	detection := adapter.Detect(ctx)
	if !detection.Installed || detection.Version != "2.1.259" {
		t.Fatalf("unexpected detection: %#v", detection)
	}
	if adapter.Compatibility().State != model.CompatibilityVerified {
		t.Fatalf("unexpected compatibility: %#v", adapter.Compatibility())
	}

	cwd := filepath.Join(fixture, "workspace")
	gitMarker := filepath.Join(cwd, ".git")
	if _, err := os.Stat(gitMarker); os.IsNotExist(err) {
		if err := os.Mkdir(gitMarker, 0o755); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Remove(gitMarker) })
	} else if err != nil {
		t.Fatal(err)
	}
	sources := adapter.DiscoverSources(ctx, cwd)
	parsed := make([]model.ParsedConfig, 0, len(sources))
	for _, source := range sources {
		if source.Exists {
			parsed = append(parsed, adapter.Parse(ctx, source))
		}
	}
	effective := adapter.Resolve(ctx, parsed)
	theme := findNode(effective.Nodes, "setting.theme")
	if theme == nil || theme.Attributes["value"].Display != "local" {
		t.Fatalf("local setting did not win: %#v", theme)
	}
	if !hasNode(effective.Nodes, "instruction.imported.md") {
		t.Fatalf("imported instruction missing: %#v", effective.Nodes)
	}
	if !hasNode(effective.Nodes, "mcp.playwright") || !hasNode(effective.Nodes, "hook.PreToolUse.0.0") {
		t.Fatalf("MCP or hook node missing: %#v", effective.Nodes)
	}
	if !hasNode(effective.Nodes, "skill.fixture") {
		t.Fatalf("Claude skill node missing: %#v", effective.Nodes)
	}
	if !hasEdgeType(effective.Edges, model.EdgeImports) || !hasEdgeType(effective.Edges, model.EdgeOverrides) {
		t.Fatalf("provenance edges missing: %#v", effective.Edges)
	}
	assertEdgeEndpointsExist(t, effective.Nodes, effective.Edges)
	for _, node := range effective.Nodes {
		for _, value := range node.Attributes {
			if strings.Contains(value.Display, "HARNESSSCOPE-CANARY") {
				t.Fatalf("secret leaked in node %#v", node)
			}
		}
	}
}

func TestImportCycleProducesFindingWithoutAbortingInstructions(t *testing.T) {
	fixture := fixturePath(t, "imports")
	env := discovery.NewEnvironment("linux", fixture, nil)
	adapter := New(env, secrets.NewRedactor(), WithManagedSettingsPath(filepath.Join(fixture, "managed.json")))
	source := env.InspectKnownPath(filepath.Join(fixture, "workspace", "CLAUDE.md"))
	source.ID = model.StableSourceID("claude", source.LogicalPath)
	source.Client, source.Scope, source.Format, source.Kind = "claude", model.ScopeProject, model.FormatMarkdown, "instruction"

	parsed := adapter.Parse(context.Background(), source)
	if !hasFinding(parsed.Findings, "SOURCE-0002") {
		t.Fatalf("cycle finding missing: %#v", parsed.Findings)
	}
	if len(parsed.Nodes) < 2 {
		t.Fatalf("cycle aborted all instruction parsing: %#v", parsed.Nodes)
	}
}

func TestMalformedJSONBecomesParseFinding(t *testing.T) {
	fixture := fixturePath(t, "malformed")
	env := discovery.NewEnvironment("linux", filepath.Join(fixture, "home"), nil)
	adapter := New(env, secrets.NewRedactor())
	source := env.InspectKnownPath(filepath.Join(fixture, "home", ".claude", "settings.json"))
	source.ID = model.StableSourceID("claude", source.LogicalPath)
	source.Client, source.Scope, source.Format, source.Kind = "claude", model.ScopeUser, model.FormatJSON, "settings"

	parsed := adapter.Parse(context.Background(), source)
	if !hasFinding(parsed.Findings, "PARSE-0001") {
		t.Fatalf("parse finding missing: %#v", parsed.Findings)
	}
}

func TestUnknownVersionDowngradesOverrideEvidence(t *testing.T) {
	fixture := fixturePath(t, "version-drift")
	env := discovery.NewEnvironment("linux", fixture, []string{filepath.Join(fixture, "bin")})
	adapter := New(env, secrets.NewRedactor())
	adapter.Detect(context.Background())

	parsed := []model.ParsedConfig{
		{Client: "claude", Nodes: []model.ConfigNode{{ID: "low", DisplayName: "setting.theme", AdapterConfidence: model.EvidenceConfirmed}}},
		{Client: "claude", Nodes: []model.ConfigNode{{ID: "high", DisplayName: "setting.theme", AdapterConfidence: model.EvidenceConfirmed}}},
	}
	effective := adapter.Resolve(context.Background(), parsed)
	if len(effective.Edges) != 1 || effective.Edges[0].Evidence != model.EvidenceUnknown {
		t.Fatalf("version drift was hidden: %#v", effective.Edges)
	}
}

func fixturePath(t *testing.T, name string) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "..", "fixtures", "claude", name))
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func findNode(nodes []model.ConfigNode, name string) *model.ConfigNode {
	for index := range nodes {
		if nodes[index].DisplayName == name {
			return &nodes[index]
		}
	}
	return nil
}

func hasNode(nodes []model.ConfigNode, name string) bool { return findNode(nodes, name) != nil }

func hasEdgeType(edges []model.Edge, edgeType model.EdgeType) bool {
	for _, edge := range edges {
		if edge.Type == edgeType {
			return true
		}
	}
	return false
}

func hasFinding(findings []model.Finding, ruleID string) bool {
	for _, finding := range findings {
		if finding.RuleID == ruleID {
			return true
		}
	}
	return false
}

func assertEdgeEndpointsExist(t *testing.T, nodes []model.ConfigNode, edges []model.Edge) {
	t.Helper()
	ids := make(map[string]bool, len(nodes))
	for _, node := range nodes {
		ids[node.ID] = true
	}
	for _, edge := range edges {
		if !ids[edge.From] || !ids[edge.To] {
			t.Fatalf("edge has missing endpoint: %#v", edge)
		}
	}
}
