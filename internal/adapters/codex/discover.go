package codex

import (
	"context"
	"os"
	"path/filepath"

	"github.com/Z-lab-boop/harnessscope/internal/discovery"
	"github.com/Z-lab-boop/harnessscope/internal/model"
)

func (a *Adapter) DiscoverSources(_ context.Context, cwd string) []model.ConfigSource {
	var sources []model.ConfigSource
	sources = append(sources, a.source(filepath.Join(a.env.SystemConfigDir, "config.toml"), model.ScopeSystem, model.FormatTOML, "config", "Codex system configuration"))
	sources = append(sources, a.source(filepath.Join(a.env.HomeDir, ".codex", "config.toml"), model.ScopeUser, model.FormatTOML, "config", "Codex user configuration"))
	sources = append(sources, a.chooseInstruction(filepath.Join(a.env.HomeDir, ".codex"), model.ScopeUser, "Codex global instructions"))
	sources = append(sources, a.discoverSkills(filepath.Join(a.env.HomeDir, ".codex", "skills"), model.ScopeUser)...)

	ancestors, err := discovery.WalkAncestors(cwd)
	if err != nil {
		return sources
	}
	root := projectRoot(ancestors)
	chain := rootToCWD(root, ancestors)
	for index, directory := range chain {
		scope := model.ScopeNested
		if index == 0 {
			scope = model.ScopeProject
		}
		sources = append(sources, a.source(filepath.Join(directory, ".codex", "config.toml"), scope, model.FormatTOML, "config", "Codex project configuration"))
		sources = append(sources, a.chooseInstruction(directory, scope, "Codex project instructions"))
	}
	sources = append(sources, a.discoverSkills(filepath.Join(root, ".agents", "skills"), model.ScopeProject)...)
	return deduplicateSources(sources)
}

func (a *Adapter) source(path string, scope model.Scope, format model.SourceFormat, kind, reason string) model.ConfigSource {
	source := a.env.InspectKnownPath(path)
	source.ID = model.StableSourceID(clientID, source.LogicalPath)
	source.Client = clientID
	source.Scope = scope
	source.Format = format
	source.Kind = kind
	source.DiscoveryReason = reason
	return source
}

func (a *Adapter) chooseInstruction(directory string, scope model.Scope, reason string) model.ConfigSource {
	override := filepath.Join(directory, "AGENTS.override.md")
	if _, err := os.Lstat(override); err == nil {
		return a.source(override, scope, model.FormatMarkdown, "instruction", reason+" override")
	}
	return a.source(filepath.Join(directory, "AGENTS.md"), scope, model.FormatMarkdown, "instruction", reason)
}

func (a *Adapter) discoverSkills(root string, scope model.Scope) []model.ConfigSource {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var sources []model.ConfigSource
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(root, entry.Name(), "SKILL.md")
		source := a.source(path, scope, model.FormatMarkdown, "skill", "Codex skill directory")
		if source.Exists {
			sources = append(sources, source)
		}
	}
	return sources
}

func projectRoot(ancestors []string) string {
	for _, directory := range ancestors {
		if info, err := os.Stat(filepath.Join(directory, ".git")); err == nil && info.IsDir() {
			return directory
		}
	}
	return ancestors[0]
}

func rootToCWD(root string, ancestors []string) []string {
	var reverse []string
	for _, directory := range ancestors {
		reverse = append(reverse, directory)
		if directory == root {
			break
		}
	}
	chain := make([]string, len(reverse))
	for index := range reverse {
		chain[len(reverse)-1-index] = reverse[index]
	}
	return chain
}

func deduplicateSources(input []model.ConfigSource) []model.ConfigSource {
	seen := make(map[string]struct{}, len(input))
	result := make([]model.ConfigSource, 0, len(input))
	for _, source := range input {
		if _, exists := seen[source.ID]; exists {
			continue
		}
		seen[source.ID] = struct{}{}
		result = append(result, source)
	}
	return result
}
