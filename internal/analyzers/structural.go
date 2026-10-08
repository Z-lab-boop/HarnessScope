package analyzers

import (
	"context"
	"os"
	"path/filepath"

	"github.com/Z-lab-boop/harnessscope/internal/model"
)

func analyzeDuplicateMCP(_ context.Context, input model.Analysis) []model.Finding {
	groups := make(map[string][]model.ConfigNode)
	for _, node := range input.Graph.Nodes {
		if node.Type == model.NodeMCPServer {
			groups[node.DisplayName] = append(groups[node.DisplayName], node)
		}
	}
	var findings []model.Finding
	for name, nodes := range groups {
		if len(nodes) < 2 {
			continue
		}
		findings = append(findings, finding(
			"MCP-0001", model.SeverityMedium, weakestEvidence(nodes),
			"MCP server name appears more than once",
			"The normalized server name "+name+" is declared by multiple configuration nodes.",
			"Different clients or scopes may launch different tools under the same familiar name.",
			"Compare the commands and keep the duplicate only when the divergence is intentional.", nodes...,
		))
	}
	return findings
}

func analyzePaths(_ context.Context, input model.Analysis) []model.Finding {
	var findings []model.Finding
	for _, node := range input.Graph.Nodes {
		if node.Type != model.NodeMCPServer && node.Type != model.NodeHook {
			continue
		}
		for _, field := range []string{"command", "cwd"} {
			value, exists := node.Attributes[field]
			if !exists || !value.Present || value.Display == "" || value.Display == "[REDACTED]" || !filepath.IsAbs(value.Display) {
				continue
			}
			if _, err := os.Stat(value.Display); err == nil {
				continue
			}
			findings = append(findings, finding(
				"PATH-0001", model.SeverityHigh, node.AdapterConfidence,
				"Referenced absolute path does not exist",
				field+" points to a missing filesystem object.",
				"The configured hook or MCP server cannot start from this path.",
				"Restore the target or update the configuration to an existing portable path.", node,
			))
		}
	}
	return findings
}

func analyzeDuplicateContext(_ context.Context, input model.Analysis) []model.Finding {
	byContent := make(map[string][]model.ConfigNode)
	for _, node := range input.Graph.Nodes {
		if node.Type != model.NodeInstruction {
			continue
		}
		content := node.Attributes["content"].Display
		if content != "" && content != "[REDACTED]" {
			byContent[content] = append(byContent[content], node)
		}
	}
	var findings []model.Finding
	for _, nodes := range byContent {
		if len(nodes) < 2 {
			continue
		}
		findings = append(findings, finding(
			"CONTEXT-0001", model.SeverityMedium, weakestEvidence(nodes),
			"Byte-identical instruction content loads more than once",
			"Multiple instruction nodes contain identical fixed context.",
			"The duplicate consumes context without adding guidance.",
			"Remove or consolidate the duplicate after reviewing its scope.", nodes...,
		))
	}
	return findings
}
