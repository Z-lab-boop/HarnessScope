package opencode

import (
	"context"
	"encoding/json"
	"fmt"
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
	clientID       = "opencode"
	rulesetVersion = "opencode-preview-2026-10-08"
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
	result := a.env.FindExecutable(clientID)
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
			"https://opencode.ai/docs/config/",
			"https://opencode.ai/docs/rules/",
		},
		SupportedFeatures: []string{"config", "instructions", "mcp", "source-discovery"},
	}
}

func (a *Adapter) DiscoverSources(_ context.Context, cwd string) []model.ConfigSource {
	globalRoot := filepath.Join(a.env.HomeDir, ".config", "opencode")
	sources := []model.ConfigSource{
		a.source(filepath.Join(globalRoot, "opencode.json"), model.ScopeUser, model.FormatJSON, "config", "OpenCode global configuration"),
		a.source(filepath.Join(globalRoot, "opencode.jsonc"), model.ScopeUser, model.FormatJSONC, "config", "OpenCode global JSONC configuration"),
		a.source(filepath.Join(globalRoot, "AGENTS.md"), model.ScopeUser, model.FormatMarkdown, "instruction", "OpenCode global instructions"),
	}
	root := findProjectRoot(cwd)
	sources = append(sources,
		a.source(filepath.Join(root, "opencode.json"), model.ScopeProject, model.FormatJSON, "config", "OpenCode project configuration"),
		a.source(filepath.Join(root, "opencode.jsonc"), model.ScopeProject, model.FormatJSONC, "config", "OpenCode project JSONC configuration"),
		a.source(filepath.Join(root, "AGENTS.md"), model.ScopeProject, model.FormatMarkdown, "instruction", "OpenCode project instructions"),
	)
	return deduplicate(sources)
}

func (a *Adapter) Parse(_ context.Context, source model.ConfigSource) model.ParsedConfig {
	result := model.ParsedConfig{Client: clientID, Sources: []model.ConfigSource{source}}
	data, err := os.ReadFile(source.CanonicalPath)
	if err != nil {
		result.Findings = append(result.Findings, parseFinding(source, "Source is not readable"))
		return result
	}
	if source.Format == model.FormatMarkdown {
		result.Nodes = append(result.Nodes, a.instructionNode(source, data))
		return result
	}
	cleaned := data
	if source.Format == model.FormatJSONC {
		cleaned, err = formats.NormalizeJSONC(data)
		if err != nil {
			result.Findings = append(result.Findings, parseFinding(source, "JSONC syntax is invalid"))
			return result
		}
	}
	var raw map[string]any
	if err := json.Unmarshal(cleaned, &raw); err != nil {
		result.Findings = append(result.Findings, parseFinding(source, "JSON syntax is invalid"))
		return result
	}
	result.Nodes = append(result.Nodes, a.configNodes(source, raw)...)
	return result
}

func (a *Adapter) Resolve(_ context.Context, parsed []model.ParsedConfig) model.EffectiveConfig {
	result := model.EffectiveConfig{Client: clientID, Limitations: []string{"PREVIEW adapter: documented sources are inspected, but effective merge precedence is not asserted."}}
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

func sourceForTest(env discovery.Environment, path string) model.ConfigSource {
	source := env.InspectKnownPath(path)
	source.ID = model.StableSourceID(clientID, source.LogicalPath)
	source.Client, source.Scope, source.Format, source.Kind = clientID, model.ScopeProject, model.FormatJSONC, "config"
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
		LoadCondition: "declared", AdapterConfidence: model.EvidenceUnknown,
	}
}

func (a *Adapter) configNodes(source model.ConfigSource, raw map[string]any) []model.ConfigNode {
	var nodes []model.ConfigNode
	for key, value := range raw {
		switch key {
		case "mcp":
			servers, _ := value.(map[string]any)
			for name, server := range servers {
				nodes = append(nodes, a.mcpNode(source, name, server))
			}
		case "instructions":
			items, _ := value.([]any)
			for index, item := range items {
				name := fmt.Sprint(item)
				nodes = append(nodes, model.ConfigNode{
					ID: model.StableNodeID(clientID, source.LogicalPath, fmt.Sprintf("instructions.%d", index)), Type: model.NodeInstruction,
					Client: clientID, DisplayName: "instruction.reference." + name,
					Attributes:    map[string]model.SafeValue{"path": a.redactor.RedactField(fmt.Sprintf("instructions.%d", index), item)},
					Origins:       []model.Origin{{SourceID: source.ID, LogicalPath: source.LogicalPath, FieldPath: fmt.Sprintf("instructions.%d", index), Scope: source.Scope, Rule: rulesetVersion}},
					LoadCondition: "declared_pattern", AdapterConfidence: model.EvidenceUnknown,
				})
			}
		default:
			if _, nested := value.(map[string]any); nested {
				continue
			}
			nodes = append(nodes, model.ConfigNode{
				ID: model.StableNodeID(clientID, source.LogicalPath, "setting."+key), Type: model.NodeRule,
				Client: clientID, DisplayName: "setting." + key,
				Attributes:    map[string]model.SafeValue{"value": a.redactor.RedactField(key, value)},
				Origins:       []model.Origin{{SourceID: source.ID, LogicalPath: source.LogicalPath, FieldPath: key, Scope: source.Scope, Rule: rulesetVersion}},
				LoadCondition: "declared", AdapterConfidence: model.EvidenceUnknown,
			})
		}
	}
	return nodes
}

func (a *Adapter) mcpNode(source model.ConfigSource, name string, value any) model.ConfigNode {
	attributes := make(map[string]model.SafeValue)
	if server, ok := value.(map[string]any); ok {
		for key, child := range server {
			if key == "command" {
				if command, ok := child.([]any); ok && len(command) > 0 {
					attributes[key] = a.redactor.RedactField("mcp."+name+".command", command[0])
					continue
				}
			}
			if nested, ok := child.(map[string]any); ok {
				for nestedKey, nestedValue := range nested {
					attributes[key+"."+nestedKey] = a.redactor.RedactField("mcp."+name+"."+key+"."+nestedKey, nestedValue)
				}
				continue
			}
			attributes[key] = a.redactor.RedactField("mcp."+name+"."+key, child)
		}
	} else {
		attributes["value"] = a.redactor.RedactField("mcp."+name, value)
	}
	return model.ConfigNode{
		ID: model.StableNodeID(clientID, source.LogicalPath, "mcp."+name), Type: model.NodeMCPServer,
		Client: clientID, DisplayName: "mcp." + name, Attributes: attributes,
		Origins:       []model.Origin{{SourceID: source.ID, LogicalPath: source.LogicalPath, FieldPath: "mcp." + name, Scope: source.Scope, Rule: rulesetVersion}},
		LoadCondition: "declared", AdapterConfidence: model.EvidenceUnknown,
	}
}

func parseFinding(source model.ConfigSource, reason string) model.Finding {
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
