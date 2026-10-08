package analyzers

import (
	"context"
	"testing"

	"github.com/Z-lab-boop/harnessscope/internal/model"
)

func TestRunDetectsDuplicateMCPAndMissingAbsoluteCommand(t *testing.T) {
	analysis := model.Analysis{
		Graph: model.Graph{Nodes: []model.ConfigNode{
			{ID: "a", Type: model.NodeMCPServer, Client: "codex", DisplayName: "mcp.playwright", AdapterConfidence: model.EvidenceConfirmed, Attributes: map[string]model.SafeValue{"command": {Display: "/definitely/missing/hscope-server", Present: true}}},
			{ID: "b", Type: model.NodeMCPServer, Client: "claude", DisplayName: "mcp.playwright", AdapterConfidence: model.EvidenceConfirmed, Attributes: map[string]model.SafeValue{"command": {Display: "/definitely/missing/hscope-server", Present: true}}},
		}},
	}

	findings := Run(context.Background(), analysis)
	if !containsRule(findings, "MCP-0001") {
		t.Fatalf("duplicate MCP finding missing: %#v", findings)
	}
	if !containsRule(findings, "PATH-0001") {
		t.Fatalf("missing command finding missing: %#v", findings)
	}
	for _, finding := range findings {
		if finding.Summary == "" || finding.Reason == "" || finding.Impact == "" || finding.Remediation == "" || finding.Evidence == "" {
			t.Fatalf("incomplete finding: %#v", finding)
		}
	}
}

func TestRunPropagatesUnknownEvidence(t *testing.T) {
	analysis := model.Analysis{Graph: model.Graph{Nodes: []model.ConfigNode{
		{ID: "a", Type: model.NodeMCPServer, Client: "cursor", DisplayName: "mcp.same", AdapterConfidence: model.EvidenceUnknown},
		{ID: "b", Type: model.NodeMCPServer, Client: "opencode", DisplayName: "mcp.same", AdapterConfidence: model.EvidenceConfirmed},
	}}}

	findings := Run(context.Background(), analysis)
	for _, finding := range findings {
		if finding.RuleID == "MCP-0001" && finding.Evidence != model.EvidenceUnknown {
			t.Fatalf("unknown adapter evidence was promoted: %#v", finding)
		}
	}
}

func TestEstimateContextUsesConservativeRange(t *testing.T) {
	nodes := []model.ConfigNode{{
		ID: "instruction", Client: "codex", Type: model.NodeInstruction,
		Attributes: map[string]model.SafeValue{"content": {Display: "abcdefghijkl", Present: true}},
	}}

	got := EstimateContext(nodes)
	if len(got) != 1 || got[0].CodePoints != 12 || got[0].MinTokens != 2 || got[0].MaxTokens != 6 || got[0].Method != "ESTIMATED_RANGE" {
		t.Fatalf("unexpected estimate: %#v", got)
	}
}

func containsRule(findings []model.Finding, ruleID string) bool {
	for _, finding := range findings {
		if finding.RuleID == ruleID {
			return true
		}
	}
	return false
}
