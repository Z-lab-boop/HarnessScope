package opencode

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

func TestPreviewFixtureParsesJSONCAndInstructionsWithoutPrecedenceClaims(t *testing.T) {
	fixture := fixturePath(t)
	env := discovery.NewEnvironment("darwin", filepath.Join(fixture, "home"), []string{filepath.Join(fixture, "bin")})
	adapter := New(env, secrets.NewRedactor())
	ctx := context.Background()

	if detection := adapter.Detect(ctx); !detection.Installed {
		t.Fatalf("opencode fixture was not detected: %#v", detection)
	}
	if metadata := adapter.Compatibility(); metadata.Tier != model.TierPreview || metadata.State != model.CompatibilityUnknown {
		t.Fatalf("preview boundary missing: %#v", metadata)
	}
	if adapter.Capabilities().Precedence {
		t.Fatal("preview adapter must not claim precedence support")
	}
	effective := parseAll(ctx, adapter, filepath.Join(fixture, "workspace"))
	if !hasNode(effective.Nodes, model.NodeMCPServer) || !hasNode(effective.Nodes, model.NodeInstruction) {
		t.Fatalf("expected MCP and instruction nodes: %#v", effective.Nodes)
	}
	assertNoConfirmedPrecedence(t, effective.Edges)
	assertNoCanary(t, effective.Nodes)
}

func TestMalformedJSONCBecomesSyntaxFinding(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "opencode.jsonc")
	if err := os.WriteFile(path, []byte(`{"mcp": { /* open`), 0o600); err != nil {
		t.Fatal(err)
	}
	env := discovery.NewEnvironment("linux", root, nil)
	adapter := New(env, secrets.NewRedactor())
	source := sourceForTest(env, path)
	parsed := adapter.Parse(context.Background(), source)
	if len(parsed.Findings) != 1 || parsed.Findings[0].RuleID != "PARSE-0001" || !strings.Contains(parsed.Findings[0].Reason, "JSONC") {
		t.Fatalf("syntax finding missing: %#v", parsed.Findings)
	}
}

func fixturePath(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "..", "fixtures", "opencode", "preview"))
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func parseAll(ctx context.Context, adapter *Adapter, cwd string) model.EffectiveConfig {
	var parsed []model.ParsedConfig
	for _, source := range adapter.DiscoverSources(ctx, cwd) {
		if source.Exists && source.Readable {
			parsed = append(parsed, adapter.Parse(ctx, source))
		}
	}
	return adapter.Resolve(ctx, parsed)
}

func hasNode(nodes []model.ConfigNode, nodeType model.NodeType) bool {
	for _, node := range nodes {
		if node.Type == nodeType {
			return true
		}
	}
	return false
}

func assertNoConfirmedPrecedence(t *testing.T, edges []model.Edge) {
	t.Helper()
	for _, edge := range edges {
		if (edge.Type == model.EdgeOverrides || edge.Type == model.EdgeShadows || edge.Type == model.EdgeEffectiveAs) && edge.Evidence == model.EvidenceConfirmed {
			t.Fatalf("unsupported confirmed precedence claim: %#v", edge)
		}
	}
}

func assertNoCanary(t *testing.T, nodes []model.ConfigNode) {
	t.Helper()
	for _, node := range nodes {
		for _, value := range node.Attributes {
			if strings.Contains(value.Display, "HARNESSSCOPE-CANARY") {
				t.Fatalf("secret leaked: %#v", node)
			}
		}
	}
}
