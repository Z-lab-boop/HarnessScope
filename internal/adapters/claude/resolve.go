package claude

import (
	"context"
	"sort"

	"github.com/Z-lab-boop/harnessscope/internal/model"
)

func (a *Adapter) Resolve(_ context.Context, parsed []model.ParsedConfig) model.EffectiveConfig {
	result := model.EffectiveConfig{Client: clientID}
	winners := make(map[string]model.ConfigNode)
	var additive []model.ConfigNode
	evidence := model.EvidenceUnknown
	if a.Compatibility().State == model.CompatibilityVerified {
		evidence = model.EvidenceConfirmed
	}
	for _, config := range parsed {
		result.Sources = append(result.Sources, config.Sources...)
		result.Findings = append(result.Findings, config.Findings...)
		result.Edges = append(result.Edges, config.Edges...)
		for _, node := range config.Nodes {
			if node.Type == model.NodeInstruction || node.Type == model.NodeSkill {
				additive = append(additive, node)
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
		result.Nodes = append(result.Nodes, winners[key])
	}
	sort.Slice(additive, func(i, j int) bool { return additive[i].ID < additive[j].ID })
	result.Nodes = append(result.Nodes, additive...)
	return result
}
