package claude

import (
	"context"
	"os"
	"path/filepath"

	"github.com/Z-lab-boop/harnessscope/internal/discovery"
	"github.com/Z-lab-boop/harnessscope/internal/model"
)

func (a *Adapter) DiscoverSources(_ context.Context, cwd string) []model.ConfigSource {
	sources := []model.ConfigSource{
		a.source(filepath.Join(a.env.HomeDir, ".claude", "settings.json"), model.ScopeUser, model.FormatJSON, "settings", "Claude user settings"),
		a.source(filepath.Join(a.env.HomeDir, ".claude.json"), model.ScopeUser, model.FormatJSON, "user_state", "Claude user and local MCP state"),
		a.source(filepath.Join(a.env.HomeDir, ".claude", "CLAUDE.md"), model.ScopeUser, model.FormatMarkdown, "instruction", "Claude user instructions"),
	}
	sources = append(sources, a.discoverSkills(filepath.Join(a.env.HomeDir, ".claude", "skills"), model.ScopeUser)...)
	ancestors, err := discovery.WalkAncestors(cwd)
	if err != nil {
		return append(sources, a.managedSource())
	}
	root := findProjectRoot(ancestors)
	for index, directory := range rootToCurrent(root, ancestors) {
		scope := model.ScopeNested
		if index == 0 {
			scope = model.ScopeProject
		}
		for _, path := range []string{filepath.Join(directory, "CLAUDE.md"), filepath.Join(directory, ".claude", "CLAUDE.md")} {
			source := a.source(path, scope, model.FormatMarkdown, "instruction", "Claude project instructions")
			if source.Exists {
				sources = append(sources, source)
			}
		}
	}
	sources = append(sources,
		a.source(filepath.Join(root, ".claude", "settings.json"), model.ScopeProject, model.FormatJSON, "settings", "Claude project settings"),
		a.source(filepath.Join(root, ".claude", "settings.local.json"), model.ScopeLocal, model.FormatJSON, "settings", "Claude local settings"),
		a.source(filepath.Join(root, ".mcp.json"), model.ScopeProject, model.FormatJSON, "mcp", "Claude project MCP configuration"),
		a.managedSource(),
	)
	sources = append(sources, a.discoverSkills(filepath.Join(root, ".claude", "skills"), model.ScopeProject)...)
	return deduplicate(sources)
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
		source := a.source(filepath.Join(root, entry.Name(), "SKILL.md"), scope, model.FormatMarkdown, "skill", "Claude skill directory")
		if source.Exists {
			sources = append(sources, source)
		}
	}
	return sources
}

func (a *Adapter) source(path string, scope model.Scope, format model.SourceFormat, kind, reason string) model.ConfigSource {
	source := a.env.InspectKnownPath(path)
	source.ID = model.StableSourceID(clientID, source.LogicalPath)
	source.Client, source.Scope, source.Format = clientID, scope, format
	source.Kind, source.DiscoveryReason = kind, reason
	return source
}

func findProjectRoot(ancestors []string) string {
	for _, directory := range ancestors {
		if info, err := os.Stat(filepath.Join(directory, ".git")); err == nil && info.IsDir() {
			return directory
		}
	}
	return ancestors[0]
}

func rootToCurrent(root string, ancestors []string) []string {
	var reverse []string
	for _, directory := range ancestors {
		reverse = append(reverse, directory)
		if directory == root {
			break
		}
	}
	result := make([]string, len(reverse))
	for index := range reverse {
		result[len(reverse)-1-index] = reverse[index]
	}
	return result
}

func deduplicate(sources []model.ConfigSource) []model.ConfigSource {
	seen := make(map[string]struct{}, len(sources))
	result := make([]model.ConfigSource, 0, len(sources))
	for _, source := range sources {
		if _, ok := seen[source.ID]; ok {
			continue
		}
		seen[source.ID] = struct{}{}
		result = append(result, source)
	}
	return result
}
