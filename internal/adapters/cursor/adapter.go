package cursor

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/Z-lab-boop/harnessscope/internal/discovery"
	"github.com/Z-lab-boop/harnessscope/internal/formats"
	"github.com/Z-lab-boop/harnessscope/internal/model"
	"github.com/Z-lab-boop/harnessscope/internal/secrets"
)

const (
	clientID       = "cursor"
	rulesetVersion = "cursor-preview-2026-10-08"
)

type Adapter struct {
	env      discovery.Environment
	redactor secrets.Redactor
	mu       sync.RWMutex
	detected model.DetectionResult
}

func New(env discovery.Environment, redactor secrets.Redactor) *Adapter {
	return &Adapter{env: env, redactor: redactor}
}

func (a *Adapter) ID() string { return clientID }

func (a *Adapter) Detect(ctx context.Context) model.DetectionResult {
	result := a.env.FindExecutable("cursor")
	if !result.Installed {
		result = a.env.FindExecutable("agent")
	}
	if result.Installed {
		versionCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		output, err := exec.CommandContext(versionCtx, result.Executable, "--version").CombinedOutput()
		if err != nil {
			result.Error = a.redactor.ScrubText(err.Error())
		} else {
			result.Version = lastField(string(output))
		}
	}
	a.mu.Lock()
	a.detected = result
	a.mu.Unlock()
	return result
}

func (a *Adapter) Capabilities() model.AdapterCapabilities {
	return model.AdapterCapabilities{Instructions: true, MCP: true}
}

func (a *Adapter) Compatibility() model.CompatibilityMetadata {
	a.mu.RLock()
	detection := a.detected
	a.mu.RUnlock()
	return model.CompatibilityMetadata{
		Tier: model.TierPreview, State: model.CompatibilityUnknown, InstalledVersion: detection.Version,
		RulesetVersion: rulesetVersion, LastVerificationAt: "2026-10-08",
		EvidenceReferences: []string{
			"https://cursor.com/docs/context/rules",
			"https://cursor.com/help/customization/mcp",
		},
		SupportedFeatures: []string{"rules", "mcp", "source-discovery"},
	}
}

func (a *Adapter) DiscoverSources(_ context.Context, cwd string) []model.ConfigSource {
	sources := []model.ConfigSource{
		a.source(filepath.Join(a.env.HomeDir, ".cursor", "mcp.json"), model.ScopeUser, model.FormatJSON, "mcp", "Cursor global MCP configuration"),
	}
	root := findProjectRoot(cwd)
	for _, directory := range rootToCWD(root, cwd) {
		scope := model.ScopeNested
		if directory == root {
			scope = model.ScopeProject
		}
		sources = append(sources,
			a.source(filepath.Join(directory, ".cursor", "mcp.json"), scope, model.FormatJSON, "mcp", "Cursor project MCP configuration"),
			a.source(filepath.Join(directory, ".cursorrules"), scope, model.FormatMarkdown, "instruction", "Cursor legacy project rules"),
		)
		rulesRoot := filepath.Join(directory, ".cursor", "rules")
		_ = filepath.WalkDir(rulesRoot, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if !entry.IsDir() && strings.EqualFold(filepath.Ext(path), ".mdc") {
				sources = append(sources, a.source(path, scope, model.FormatMarkdown, "instruction", "Cursor project rule"))
			}
			return nil
		})
	}
	return deduplicate(sources)
}

func (a *Adapter) Parse(_ context.Context, source model.ConfigSource) model.ParsedConfig {
	result := model.ParsedConfig{Client: clientID, Sources: []model.ConfigSource{source}}
	data, err := os.ReadFile(source.CanonicalPath)
	if err != nil {
		result.Findings = append(result.Findings, finding(source, "Source is not readable"))
		return result
	}
	if source.Format == model.FormatMarkdown {
		result.Nodes = append(result.Nodes, a.instructionNode(source, data))
		return result
	}
	cleaned, err := formats.NormalizeJSONC(data)
	if err != nil {
		result.Findings = append(result.Findings, finding(source, "JSONC syntax is invalid"))
		return result
	}
	var raw map[string]any
	if err := json.Unmarshal(cleaned, &raw); err != nil {
		result.Findings = append(result.Findings, finding(source, "JSON syntax is invalid"))
		return result
	}
	servers, _ := raw["mcpServers"].(map[string]any)
	for name, value := range servers {
		result.Nodes = append(result.Nodes, a.mcpNode(source, name, value))
	}
	return result
}

func (a *Adapter) Resolve(_ context.Context, parsed []model.ParsedConfig) model.EffectiveConfig {
	result := model.EffectiveConfig{Client: clientID, Limitations: []string{"PREVIEW adapter: effective precedence and shadowing are not asserted."}}
	for _, config := range parsed {
		result.Sources = append(result.Sources, config.Sources...)
		result.Nodes = append(result.Nodes, config.Nodes...)
		result.Findings = append(result.Findings, config.Findings...)
		result.Limitations = append(result.Limitations, config.Limitations...)
	}
	sort.Slice(result.Nodes, func(i, j int) bool { return result.Nodes[i].ID < result.Nodes[j].ID })
	return result
}

func (a *Adapter) source(path string, scope model.Scope, format model.SourceFormat, kind, reason string) model.ConfigSource {
	source := a.env.InspectKnownPath(path)
	source.ID = model.StableSourceID(clientID, source.LogicalPath)
	source.Client, source.Scope, source.Format, source.Kind, source.DiscoveryReason = clientID, scope, format, kind, reason
	return source
}

func (a *Adapter) instructionNode(source model.ConfigSource, data []byte) model.ConfigNode {
	return model.ConfigNode{
		ID: model.StableNodeID(clientID, source.LogicalPath, "instruction"), Type: model.NodeInstruction,
		Client: clientID, DisplayName: "instruction." + filepath.Base(source.LogicalPath),
		Attributes: map[string]model.SafeValue{
			"content":     a.redactor.RedactField("content", string(data)),
			"bytes":       a.redactor.RedactField("bytes", len(data)),
			"code_points": a.redactor.RedactField("code_points", utf8.RuneCount(data)),
		},
		Origins:       []model.Origin{{SourceID: source.ID, LogicalPath: source.LogicalPath, Scope: source.Scope, Rule: rulesetVersion}},
		LoadCondition: "cursor_rule_metadata", AdapterConfidence: model.EvidenceUnknown,
	}
}

func (a *Adapter) mcpNode(source model.ConfigSource, name string, value any) model.ConfigNode {
	attributes := make(map[string]model.SafeValue)
	if server, ok := value.(map[string]any); ok {
		for key, child := range server {
			if nested, ok := child.(map[string]any); ok {
				for nestedKey, nestedValue := range nested {
					attributes[key+"."+nestedKey] = a.redactor.RedactField("mcpServers."+name+"."+key+"."+nestedKey, nestedValue)
				}
				continue
			}
			attributes[key] = a.redactor.RedactField("mcpServers."+name+"."+key, child)
		}
	} else {
		attributes["value"] = a.redactor.RedactField("mcpServers."+name, value)
	}
	return model.ConfigNode{
		ID: model.StableNodeID(clientID, source.LogicalPath, "mcp."+name), Type: model.NodeMCPServer,
		Client: clientID, DisplayName: "mcp." + name, Attributes: attributes,
		Origins:       []model.Origin{{SourceID: source.ID, LogicalPath: source.LogicalPath, FieldPath: "mcpServers." + name, Scope: source.Scope, Rule: rulesetVersion}},
		LoadCondition: "declared", AdapterConfidence: model.EvidenceUnknown,
	}
}

func finding(source model.ConfigSource, reason string) model.Finding {
	return model.Finding{
		RuleID: "PARSE-0001", Severity: model.SeverityHigh, Evidence: model.EvidenceUnknown,
		Summary: "Preview configuration could not be parsed", Reason: reason,
		Impact:          "HarnessScope cannot inspect declarations in this source.",
		AffectedClients: []string{clientID}, Origins: []model.Origin{{SourceID: source.ID, LogicalPath: source.LogicalPath, Scope: source.Scope, Rule: rulesetVersion}},
		Remediation: "Correct the syntax and scan again.",
	}
}

func findProjectRoot(cwd string) string {
	absolute, err := filepath.Abs(cwd)
	if err != nil {
		return filepath.Clean(cwd)
	}
	ancestors, err := discovery.WalkAncestors(absolute)
	if err == nil {
		for _, directory := range ancestors {
			if info, statErr := os.Stat(filepath.Join(directory, ".git")); statErr == nil && info.IsDir() {
				return directory
			}
		}
	}
	return absolute
}

func rootToCWD(root, cwd string) []string {
	absolute, err := filepath.Abs(cwd)
	if err != nil {
		return []string{root}
	}
	var reverse []string
	for current := absolute; ; current = filepath.Dir(current) {
		reverse = append(reverse, current)
		if current == root || filepath.Dir(current) == current {
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
	seen := make(map[string]bool)
	result := make([]model.ConfigSource, 0, len(sources))
	for _, source := range sources {
		if !seen[source.ID] {
			seen[source.ID] = true
			result = append(result, source)
		}
	}
	return result
}

func lastField(value string) string {
	fields := strings.Fields(strings.TrimSpace(value))
	if len(fields) == 0 {
		return ""
	}
	return fields[len(fields)-1]
}
