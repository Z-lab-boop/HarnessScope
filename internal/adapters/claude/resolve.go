package claude

import (
	"context"
	"sort"

	"github.com/Z-lab-boop/harnessscope/internal/model"
)

func (a *Adapter) Resolve(_ context.Context, parsed []model.ParsedConfig) model.EffectiveConfig {
	result := model.EffectiveConfig{Client: clientID}
	winners := make(map[string]model.ConfigNode)
	allNodes := make(map[string]model.ConfigNode)
	evidence := model.EvidenceUnknown
	if a.Compatibility().State == model.CompatibilityVerified {
		evidence = model.EvidenceConfirmed
	}
	for _, config := range parsed {
		result.Sources = append(result.Sources, config.Sources...)
		result.Findings = append(result.Findings, config.Findings...)
		result.Edges = append(result.Edges, config.Edges...)
		for _, node := range config.Nodes {
			allNodes[node.ID] = node
			if node.Type == model.NodeInstruction || node.Type == model.NodeSkill {
				continue
			}
			if previous, exists := winners[node.DisplayName]; exists {
				result.Edges = append(result.Edges, model.Edge{
					ID:   model.StableEdgeID(node.ID, model.EdgeOverrides, previous.ID),
					From: node.ID, To: previous.ID, Type: model.EdgeOverrides,
					Ruleset: rulesetVersion, Evidence: evidence,
				})
			}
			winners[node.DisplayName] = node
		}
	}
	keys := make([]string, 0, len(winners))
	for key := range winners {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		node := winners[key]
		result.Nodes = append(result.Nodes, node)
		delete(allNodes, node.ID)
	}
	remaining := make([]model.ConfigNode, 0, len(allNodes))
	for _, node := range allNodes {
		remaining = append(remaining, node)
	}
	sort.Slice(remaining, func(i, j int) bool { return remaining[i].ID < remaining[j].ID })
	result.Nodes = append(result.Nodes, remaining...)
	return result
}
