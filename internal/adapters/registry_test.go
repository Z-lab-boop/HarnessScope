package adapters

import (
	"context"
	"testing"

	"github.com/Z-lab-boop/harnessscope/internal/model"
)

type fakeAdapter struct{ id string }

func (f fakeAdapter) ID() string { return f.id }
func (f fakeAdapter) Detect(context.Context) model.DetectionResult {
	return model.DetectionResult{}
}
func (f fakeAdapter) DiscoverSources(context.Context, string) []model.ConfigSource { return nil }
func (f fakeAdapter) Parse(context.Context, model.ConfigSource) model.ParsedConfig {
	return model.ParsedConfig{}
}
func (f fakeAdapter) Resolve(context.Context, []model.ParsedConfig) model.EffectiveConfig {
	return model.EffectiveConfig{}
}
func (f fakeAdapter) Capabilities() model.AdapterCapabilities { return model.AdapterCapabilities{} }
func (f fakeAdapter) Compatibility() model.CompatibilityMetadata {
	return model.CompatibilityMetadata{}
}

func TestRegistryRejectsDuplicateClientID(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register(fakeAdapter{id: "codex"}); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(fakeAdapter{id: "codex"}); err == nil {
		t.Fatal("expected duplicate client ID error")
	}
}

func TestSelectAllUsesStableClientOrder(t *testing.T) {
	registry := NewRegistry(fakeAdapter{id: "opencode"}, fakeAdapter{id: "codex"})

	got, err := registry.Select([]string{"all"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID() != "codex" || got[1].ID() != "opencode" {
		t.Fatalf("unexpected selection order: %#v", got)
	}
}

func TestSelectRejectsUnknownClient(t *testing.T) {
	registry := NewRegistry(fakeAdapter{id: "codex"})

	if _, err := registry.Select([]string{"claude"}); err == nil {
		t.Fatal("expected unknown client error")
	}
}
