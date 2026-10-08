package cli

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Z-lab-boop/harnessscope/internal/adapters"
	"github.com/Z-lab-boop/harnessscope/internal/adapters/claude"
	"github.com/Z-lab-boop/harnessscope/internal/adapters/codex"
	"github.com/Z-lab-boop/harnessscope/internal/adapters/cursor"
	"github.com/Z-lab-boop/harnessscope/internal/adapters/opencode"
	"github.com/Z-lab-boop/harnessscope/internal/analyzers"
	"github.com/Z-lab-boop/harnessscope/internal/discovery"
	"github.com/Z-lab-boop/harnessscope/internal/model"
	"github.com/Z-lab-boop/harnessscope/internal/resolver"
	"github.com/Z-lab-boop/harnessscope/internal/secrets"
	"github.com/Z-lab-boop/harnessscope/internal/server"
)

type Runtime struct {
	Registry    *adapters.Registry
	Environment discovery.Environment
	OpenPath    func(context.Context, string) error
	OpenBrowser func(context.Context, string) error
	StartServer func(context.Context, server.HTTPConfig) (ServerInstance, error)
	ToolVersion string
}

type ServerInstance interface {
	URL() string
	Wait() error
	Close(context.Context) error
}

type ScanOptions struct {
	CWD      string
	Clients  []string
	NoReport bool
	FailOn   model.Severity
}

func DefaultRuntime() (*Runtime, error) {
	env, err := discovery.CurrentEnvironment()
	if err != nil {
		return nil, fmt.Errorf("detect local environment: %w", err)
	}
	redactor := secrets.NewRedactor()
	return &Runtime{
		Registry: adapters.NewRegistry(
			codex.New(env, redactor), claude.New(env, redactor), cursor.New(env, redactor), opencode.New(env, redactor),
		),
		Environment: env,
		OpenPath:    platformOpen(env.GOOS),
		OpenBrowser: platformOpen(env.GOOS),
		StartServer: startServer,
		ToolVersion: "0.2.0",
	}, nil
}

func startServer(ctx context.Context, config server.HTTPConfig) (ServerInstance, error) {
	return server.Start(ctx, config)
}

func platformOpen(goos string) func(context.Context, string) error {
	return func(ctx context.Context, path string) error {
		command := "xdg-open"
		if goos == "darwin" {
			command = "open"
		}
		return exec.CommandContext(ctx, command, path).Run()
	}
}

func (r *Runtime) Scan(ctx context.Context, options ScanOptions) (model.ScanResult, error) {
	cwd, err := filepath.Abs(options.CWD)
	if err != nil {
		return model.ScanResult{}, fmt.Errorf("resolve scan path: %w", err)
	}
	selected, err := r.Registry.Select(options.Clients)
	if err != nil {
		return model.ScanResult{}, err
	}
	clients := make([]model.ClientResult, 0, len(selected))
	var sources []model.ConfigSource
	var findings []model.Finding
	for _, adapter := range selected {
		detection := adapter.Detect(ctx)
		client := model.ClientResult{
			ID: adapter.ID(), Detection: detection, Capabilities: adapter.Capabilities(),
			Compatibility: adapter.Compatibility(), Effective: model.EffectiveConfig{Client: adapter.ID()},
		}
		if detection.Installed {
			discovered := adapter.DiscoverSources(ctx, cwd)
			parsed := make([]model.ParsedConfig, 0, len(discovered))
			for _, source := range discovered {
				if source.Exists && source.Readable {
					parsed = append(parsed, adapter.Parse(ctx, source))
				}
			}
			client.Effective = adapter.Resolve(ctx, parsed)
			client.Effective.Sources = mergeSources(discovered, client.Effective.Sources)
		}
		clients = append(clients, client)
		sources = append(sources, client.Effective.Sources...)
		findings = append(findings, client.Effective.Findings...)
	}
	analysis := model.Analysis{Clients: clients, Sources: mergeSources(sources, nil)}
	analysis.Graph = resolver.Build(clients)
	if err := resolver.Validate(analysis.Graph); err != nil {
		return model.ScanResult{}, fmt.Errorf("validate provenance graph: %w", err)
	}
	analysis.Findings = append(findings, analyzers.RunWithOptions(ctx, analysis, analyzers.Options{
		PathEntries: r.Environment.PathEntries, ScanRoot: cwd, HomeDir: r.Environment.HomeDir,
	})...)
	analysis.Context = analyzers.EstimateContext(analysis.Graph.Nodes)
	return model.Canonicalize(model.ScanResult{
		SchemaVersion: model.ReportSchemaVersion,
		Analysis:      analysis,
		RunMetadata:   &model.RunMetadata{CWD: collapseHome(cwd, r.Environment.HomeDir), OS: r.Environment.GOOS},
	}), nil
}

func mergeSources(first, second []model.ConfigSource) []model.ConfigSource {
	byID := make(map[string]model.ConfigSource, len(first)+len(second))
	combined := append(append([]model.ConfigSource(nil), first...), second...)
	for _, source := range combined {
		byID[source.ID] = source
	}
	result := make([]model.ConfigSource, 0, len(byID))
	for _, source := range byID {
		result = append(result, source)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func collapseHome(path, home string) string {
	cleanPath, cleanHome := filepath.Clean(path), filepath.Clean(home)
	if cleanPath == cleanHome {
		return "~"
	}
	if strings.HasPrefix(cleanPath, cleanHome+string(filepath.Separator)) {
		return "~" + strings.TrimPrefix(cleanPath, cleanHome)
	}
	return cleanPath
}
