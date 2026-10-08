package codex

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"

	"github.com/Z-lab-boop/harnessscope/internal/model"
)

func (a *Adapter) Parse(_ context.Context, source model.ConfigSource) model.ParsedConfig {
	parsed := model.ParsedConfig{Client: clientID, Sources: []model.ConfigSource{source}}
	data, err := os.ReadFile(source.CanonicalPath)
	if err != nil {
		parsed.Findings = append(parsed.Findings, sourceFinding(source, "Source is not readable"))
		return parsed
	}
	if source.Format == model.FormatTOML {
		return a.parseTOML(source, data)
	}
	if source.Format == model.FormatMarkdown {
		return a.parseMarkdown(source, data)
	}
	parsed.Findings = append(parsed.Findings, parseFinding(source, "Unsupported Codex source format"))
	return parsed
}

func (a *Adapter) parseTOML(source model.ConfigSource, data []byte) model.ParsedConfig {
	result := model.ParsedConfig{Client: clientID, Sources: []model.ConfigSource{source}}
	var raw map[string]any
	if err := toml.Unmarshal(data, &raw); err != nil {
		result.Findings = append(result.Findings, parseFinding(source, "TOML syntax is invalid"))
		return result
	}
	for key, value := range raw {
		switch key {
		case "mcp_servers":
			servers, ok := value.(map[string]any)
			if !ok {
				continue
			}
			for name, serverValue := range servers {
				attributes := a.safeAttributes("mcp_servers."+name, serverValue)
				result.Nodes = append(result.Nodes, newNode(source, model.NodeMCPServer, "mcp."+name, attributes, "always"))
			}
		case "environment", "env":
			values, ok := value.(map[string]any)
			if !ok {
				continue
			}
			for name, envValue := range values {
				field := key + "." + name
				result.Nodes = append(result.Nodes, newNode(source, model.NodeEnvironment, "env."+name, map[string]model.SafeValue{
					"value": a.redactor.RedactField(field, envValue),
				}, "environment_present"))
			}
		default:
			if _, nested := value.(map[string]any); nested {
				continue
			}
			result.Nodes = append(result.Nodes, newNode(source, model.NodeRule, "setting."+key, map[string]model.SafeValue{
				"value": a.redactor.RedactField(key, value),
			}, "always"))
		}
	}
	return result
}

func (a *Adapter) parseMarkdown(source model.ConfigSource, data []byte) model.ParsedConfig {
	result := model.ParsedConfig{Client: clientID, Sources: []model.ConfigSource{source}}
	nodeType := model.NodeInstruction
	displayName := "instruction." + source.ID
	attributes := map[string]model.SafeValue{
		"content":     a.redactor.RedactField("content", string(data)),
		"bytes":       a.redactor.RedactField("bytes", len(data)),
		"code_points": a.redactor.RedactField("code_points", utf8.RuneCount(data)),
	}
	if source.Kind == "skill" {
		nodeType = model.NodeSkill
		displayName = "skill." + filepath.Base(filepath.Dir(source.LogicalPath))
		if metadata := parseFrontMatter(data); metadata != nil {
			for key, value := range metadata {
				attributes[key] = a.redactor.RedactField("frontmatter."+key, value)
			}
		}
	}
	result.Nodes = append(result.Nodes, newNode(source, nodeType, displayName, attributes, "always"))
	return result
}

func (a *Adapter) safeAttributes(prefix string, value any) map[string]model.SafeValue {
	attributes := make(map[string]model.SafeValue)
	values, ok := value.(map[string]any)
	if !ok {
		attributes["value"] = a.redactor.RedactField(prefix, value)
		return attributes
	}
	for key, child := range values {
		if nested, ok := child.(map[string]any); ok {
			for nestedKey, nestedValue := range nested {
				attributes[key+"."+nestedKey] = a.redactor.RedactField(prefix+"."+key+"."+nestedKey, nestedValue)
			}
			continue
		}
		attributes[key] = a.redactor.RedactField(prefix+"."+key, child)
	}
	return attributes
}

func newNode(source model.ConfigSource, nodeType model.NodeType, displayName string, attributes map[string]model.SafeValue, condition string) model.ConfigNode {
	confidence := model.EvidenceConfirmed
	if source.Kind == "config" && (source.Scope == model.ScopeProject || source.Scope == model.ScopeNested) {
		condition = "trusted_project"
		confidence = model.EvidenceLikely
	}
	return model.ConfigNode{
		ID:                model.StableNodeID(clientID, source.LogicalPath, displayName),
		Type:              nodeType,
		Client:            clientID,
		DisplayName:       displayName,
		Attributes:        attributes,
		Origins:           []model.Origin{{SourceID: source.ID, LogicalPath: source.LogicalPath, Scope: source.Scope, FieldPath: displayName, Rule: rulesetVersion}},
		LoadCondition:     condition,
		AdapterConfidence: confidence,
	}
}

func parseFrontMatter(data []byte) map[string]any {
	text := string(data)
	if !strings.HasPrefix(text, "---\n") {
		return nil
	}
	end := strings.Index(text[4:], "\n---")
	if end < 0 {
		return nil
	}
	var metadata map[string]any
	if err := yaml.Unmarshal([]byte(text[4:4+end]), &metadata); err != nil {
		return nil
	}
	return metadata
}

func parseFinding(source model.ConfigSource, reason string) model.Finding {
	return model.Finding{
		RuleID: "PARSE-0001", Severity: model.SeverityHigh, Evidence: model.EvidenceConfirmed,
		Summary: "Configuration could not be parsed", Reason: reason,
		Impact:          "Values from this source cannot contribute to the effective configuration.",
		Origins:         []model.Origin{{SourceID: source.ID, LogicalPath: source.LogicalPath, Scope: source.Scope, Rule: rulesetVersion}},
		AffectedClients: []string{clientID}, Remediation: "Correct the syntax and scan again.",
	}
}

func sourceFinding(source model.ConfigSource, reason string) model.Finding {
	return model.Finding{
		RuleID: "SOURCE-0001", Severity: model.SeverityHigh, Evidence: model.EvidenceConfirmed,
		Summary: "Configuration source is unreadable", Reason: reason,
		Impact:          "HarnessScope cannot inspect this source.",
		Origins:         []model.Origin{{SourceID: source.ID, LogicalPath: source.LogicalPath, Scope: source.Scope, Rule: rulesetVersion}},
		AffectedClients: []string{clientID}, Remediation: fmt.Sprintf("Restore read access to %s and scan again.", source.LogicalPath),
	}
}
