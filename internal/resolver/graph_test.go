package resolver

import (
	"encoding/json"
	"testing"

	"github.com/Z-lab-boop/harnessscope/internal/model"
)

func TestBuildIsStableAcrossClientInputOrder(t *testing.T) {
	a := model.ClientResult{ID: "codex", Effective: model.EffectiveConfig{Client: "codex", Nodes: []model.ConfigNode{{ID: "node_b"}}}}
	b := model.ClientResult{ID: "claude", Effective: model.EffectiveConfig{Client: "claude", Nodes: []model.ConfigNode{{ID: "node_a"}}}}

	first, _ := json.Marshal(Build([]model.ClientResult{a, b}))
	second, _ := json.Marshal(Build([]model.ClientResult{b, a}))
	if string(first) != string(second) {
		t.Fatalf("graph changed with input order\n%s\n%s", first, second)
	}
}

func TestValidateAllowsCyclesButRejectsDanglingReferences(t *testing.T) {
	cyclic := model.Graph{
		Nodes: []model.ConfigNode{{ID: "a"}, {ID: "b"}},
		Edges: []model.Edge{{ID: "ab", From: "a", To: "b"}, {ID: "ba", From: "b", To: "a"}},
	}
	if err := Validate(cyclic); err != nil {
		t.Fatalf("cycle should be representable: %v", err)
	}
	dangling := model.Graph{Nodes: []model.ConfigNode{{ID: "a"}}, Edges: []model.Edge{{ID: "missing", From: "a", To: "b"}}}
	if err := Validate(dangling); err == nil {
		t.Fatal("expected dangling-reference error")
	}
}
