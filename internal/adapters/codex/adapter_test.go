package codex

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Z-lab-boop/harnessscope/internal/discovery"
	"github.com/Z-lab-boop/harnessscope/internal/model"
	"github.com/Z-lab-boop/harnessscope/internal/secrets"
)

func TestVerifiedFixtureResolvesProjectConfigurationAndInstructionChain(t *testing.T) {
	fixture := fixturePath(t, "basic")
	env := discovery.NewEnvironment("darwin", filepath.Join(fixture, "home"), []string{filepath.Join(fixture, "bin")})
	env.SystemConfigDir = filepath.Join(fixture, "system")
	adapter := New(env, secrets.NewRedactor())
	ctx := context.Background()

	detection := adapter.Detect(ctx)
	if !detection.Installed || detection.Version != "0.162.0-alpha.2" {
		t.Fatalf("unexpected detection: %#v", detection)
	}
	if got := adapter.Compatibility().State; got != model.CompatibilityVerified {
		t.Fatalf("compatibility=%s", got)
	}

	cwd := filepath.Join(fixture, "workspace", "nested")
	sources := adapter.DiscoverSources(ctx, cwd)
	if !hasLogicalSuffix(sources, "home/.codex/config.toml") ||
		!hasLogicalSuffix(sources, "workspace/.codex/config.toml") ||
		!hasLogicalSuffix(sources, "workspace/AGENTS.md") ||
		!hasLogicalSuffix(sources, "nested/AGENTS.override.md") ||
		!hasLogicalSuffix(sources, "skills/security/SKILL.md") {
		t.Fatalf("expected sources were not discovered: %#v", sources)
	}
	if hasLogicalSuffix(sources, "nested/AGENTS.md") {
		t.Fatal("AGENTS.md was loaded despite AGENTS.override.md")
	}

	parsed := make([]model.ParsedConfig, 0, len(sources))
	for _, source := range sources {
		if source.Exists {
			parsed = append(parsed, adapter.Parse(ctx, source))
		}
	}
	effective := adapter.Resolve(ctx, parsed)
	modelNode := findNode(effective.Nodes, "setting.model")
	if modelNode == nil || modelNode.Attributes["value"].Display != "project-model" {
		t.Fatalf("project model did not win: %#v", modelNode)
	}
	if modelNode.AdapterConfidence != model.EvidenceLikely || modelNode.LoadCondition != "trusted_project" {
		t.Fatalf("project trust uncertainty was hidden: %#v", modelNode)
	}
	if !hasEdgeType(effective.Edges, model.EdgeOverrides) {
		t.Fatalf("missing override edge: %#v", effective.Edges)
	}
	for _, node := range effective.Nodes {
		for _, value := range node.Attributes {
			if strings.Contains(value.Display, "HARNESSSCOPE-CANARY") {
				t.Fatalf("secret leaked in node %#v", node)
			}
		}
	}
}

func TestMalformedTOMLBecomesParseFindingWithoutRawContent(t *testing.T) {
	fixture := fixturePath(t, "malformed")
	env := discovery.NewEnvironment("linux", filepath.Join(fixture, "home"), nil)
	adapter := New(env, secrets.NewRedactor())
	source := env.InspectKnownPath(filepath.Join(fixture, "home", ".codex", "config.toml"))
	source.Client, source.Scope, source.Format, source.Kind = "codex", model.ScopeUser, model.FormatTOML, "config"
	source.ID = model.StableSourceID("codex", source.LogicalPath)

	parsed := adapter.Parse(context.Background(), source)
	if len(parsed.Findings) != 1 || parsed.Findings[0].RuleID != "PARSE-0001" {
		t.Fatalf("unexpected findings: %#v", parsed.Findings)
	}
	if strings.Contains(parsed.Findings[0].Reason, "unterminated") {
		t.Fatalf("parse finding exposed source content: %#v", parsed.Findings[0])
	}
}

func TestUnknownVersionSuppressesConfirmedPrecedence(t *testing.T) {
	fixture := fixturePath(t, "version-drift")
	env := discovery.NewEnvironment("linux", fixture, []string{filepath.Join(fixture, "bin")})
	adapter := New(env, secrets.NewRedactor())
	adapter.Detect(context.Background())

	if got := adapter.Compatibility().State; got != model.CompatibilityUnknown {
		t.Fatalf("compatibility=%s", got)
	}
	parsed := []model.ParsedConfig{
		{Client: "codex", Nodes: []model.ConfigNode{{ID: "low", DisplayName: "setting.model", AdapterConfidence: model.EvidenceConfirmed}}},
		{Client: "codex", Nodes: []model.ConfigNode{{ID: "high", DisplayName: "setting.model", AdapterConfidence: model.EvidenceConfirmed}}},
	}
	effective := adapter.Resolve(context.Background(), parsed)
	for _, edge := range effective.Edges {
		if edge.Type == model.EdgeOverrides && edge.Evidence == model.EvidenceConfirmed {
			t.Fatalf("unknown version made confirmed precedence claim: %#v", edge)
		}
	}
}

func fixturePath(t *testing.T, name string) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "..", "fixtures", "codex", name))
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func hasLogicalSuffix(sources []model.ConfigSource, suffix string) bool {
	for _, source := range sources {
		if strings.HasSuffix(filepath.ToSlash(source.LogicalPath), suffix) {
			return true
		}
	}
	return false
}

func findNode(nodes []model.ConfigNode, displayName string) *model.ConfigNode {
	for index := range nodes {
		if nodes[index].DisplayName == displayName {
			return &nodes[index]
		}
	}
	return nil
}

func hasEdgeType(edges []model.Edge, edgeType model.EdgeType) bool {
	for _, edge := range edges {
		if edge.Type == edgeType {
			return true
		}
	}
	return false
}
