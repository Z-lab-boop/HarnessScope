package resolver

import (
	"testing"

	"github.com/Z-lab-boop/harnessscope/internal/model"
)

func FuzzBuildAlwaysProducesValidReferences(f *testing.F) {
	f.Add("client", "source", "node")
	f.Add("", "../source", "<script>")
	f.Fuzz(func(t *testing.T, clientName, sourceName, nodeName string) {
		clientID := model.StableNodeID("fuzz", "client", clientName)
		sourceID := model.StableSourceID("fuzz", sourceName)
		nodeID := model.StableNodeID("fuzz", sourceName, nodeName)
		clients := []model.ClientResult{{
			ID: clientID, Compatibility: model.CompatibilityMetadata{State: model.CompatibilityUnknown, RulesetVersion: "fuzz"},
			Effective: model.EffectiveConfig{
				Client:  clientID,
				Sources: []model.ConfigSource{{ID: sourceID, LogicalPath: sourceName, Client: clientID}},
				Nodes: []model.ConfigNode{{
					ID: nodeID, Client: clientID, Type: model.NodeRule, DisplayName: nodeName,
					Origins: []model.Origin{{SourceID: sourceID, Rule: "fuzz"}}, AdapterConfidence: model.EvidenceUnknown,
				}},
			},
		}}
		if err := Validate(Build(clients)); err != nil {
			t.Fatalf("built graph is invalid: %v", err)
		}
	})
}
