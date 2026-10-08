package adapters

import (
	"context"

	"github.com/Z-lab-boop/harnessscope/internal/model"
)

type ClientAdapter interface {
	ID() string
	Detect(ctx context.Context) model.DetectionResult
	DiscoverSources(ctx context.Context, cwd string) []model.ConfigSource
	Parse(ctx context.Context, source model.ConfigSource) model.ParsedConfig
	Resolve(ctx context.Context, sources []model.ParsedConfig) model.EffectiveConfig
	Capabilities() model.AdapterCapabilities
	Compatibility() model.CompatibilityMetadata
}
