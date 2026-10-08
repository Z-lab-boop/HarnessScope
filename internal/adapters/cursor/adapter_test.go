package cursor

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Z-lab-boop/harnessscope/internal/discovery"
	"github.com/Z-lab-boop/harnessscope/internal/model"
	"github.com/Z-lab-boop/harnessscope/internal/report"
	"github.com/Z-lab-boop/harnessscope/internal/secrets"
)

func TestPreviewFixtureDiscoversRulesAndMCPWithoutPrecedenceClaims(t *testing.T) {
	fixture := fixturePath(t)
	env := discovery.NewEnvironment("darwin", filepath.Join(fixture, "home"), []string{filepath.Join(fixture, "bin")})
	adapter := New(env, secrets.NewRedactor())
	ctx := context.Background()

	if detection := adapter.Detect(ctx); !detection.Installed {
		t.Fatalf("cursor fixture was not detected: %#v", detection)
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

func TestPreviewTierAppearsInTerminalJSONAndHTML(t *testing.T) {
	client := model.ClientResult{ID: "cursor", Compatibility: model.CompatibilityMetadata{Tier: model.TierPreview, State: model.CompatibilityUnknown}, Effective: model.EffectiveConfig{Client: "cursor"}}
	result := model.ScanResult{SchemaVersion: model.ReportSchemaVersion, Analysis: model.Analysis{Clients: []model.ClientResult{client}}}
	for name, write := range map[string]func(*bytes.Buffer) error{
		"terminal": func(buffer *bytes.Buffer) error { return report.WriteTerminal(buffer, result) },
		"json":     func(buffer *bytes.Buffer) error { return report.WriteJSON(buffer, result) },
		"html":     func(buffer *bytes.Buffer) error { return report.WriteHTML(buffer, result) },
	} {
		var output bytes.Buffer
		if err := write(&output); err != nil || !strings.Contains(output.String(), "PREVIEW") {
			t.Fatalf("%s omitted PREVIEW: err=%v output=%s", name, err, output.String())
		}
	}
}

func fixturePath(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "..", "fixtures", "cursor", "preview"))
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
