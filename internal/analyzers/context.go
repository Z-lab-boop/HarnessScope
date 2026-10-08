package analyzers

import (
	"unicode/utf8"

	"github.com/Z-lab-boop/harnessscope/internal/model"
)

func EstimateContext(nodes []model.ConfigNode) []model.ContextEstimate {
	result := make([]model.ContextEstimate, 0)
	for _, node := range nodes {
		if node.Type != model.NodeInstruction && node.Type != model.NodeSkill && node.Type != model.NodeMCPServer {
			continue
		}
		content, exists := node.Attributes["content"]
		if !exists || !content.Present {
			continue
		}
		codePoints := utf8.RuneCountInString(content.Display)
		result = append(result, model.ContextEstimate{
			Client: node.Client, SourceID: firstSource(node), Category: contextCategory(node),
			Bytes: len([]byte(content.Display)), CodePoints: codePoints,
			MinTokens: ceilDiv(codePoints, 6), MaxTokens: ceilDiv(codePoints, 2),
			Method: "ESTIMATED_RANGE",
		})
	}
	return result
}

func firstSource(node model.ConfigNode) string {
	if len(node.Origins) == 0 {
		return ""
	}
	return node.Origins[0].SourceID
}

func contextCategory(node model.ConfigNode) string {
	switch node.Type {
	case model.NodeSkill:
		return "lazy_skill"
	case model.NodeMCPServer:
		return "mcp_schema"
	default:
		if node.LoadCondition == "startup" || node.LoadCondition == "always" {
			return "always_loaded"
		}
		return "conditional"
	}
}

func ceilDiv(value, divisor int) int {
	if value == 0 {
		return 0
	}
	return (value + divisor - 1) / divisor
}
