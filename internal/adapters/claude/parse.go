package claude

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/Z-lab-boop/harnessscope/internal/model"
)

var importPattern = regexp.MustCompile(`@([^\s` + "`" + `]+)`)

func (a *Adapter) Parse(_ context.Context, source model.ConfigSource) model.ParsedConfig {
	if source.Format == model.FormatMarkdown {
		if source.Kind == "skill" {
			return a.parseSkill(source)
		}
		return a.parseInstructionTree(source)
	}
	result := model.ParsedConfig{Client: clientID, Sources: []model.ConfigSource{source}}
	data, err := os.ReadFile(source.CanonicalPath)
	if err != nil {
		result.Findings = append(result.Findings, claudeSourceFinding(source, "Source is not readable"))
		return result
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		result.Findings = append(result.Findings, claudeParseFinding(source, "JSON syntax is invalid"))
		return result
	}
	a.parseJSONObject(&result, source, raw)
	return result
}

func (a *Adapter) parseSkill(source model.ConfigSource) model.ParsedConfig {
	result := model.ParsedConfig{Client: clientID, Sources: []model.ConfigSource{source}}
	data, err := os.ReadFile(source.CanonicalPath)
	if err != nil {
		result.Findings = append(result.Findings, claudeSourceFinding(source, "Skill is not readable"))
		return result
	}
	name := filepath.Base(filepath.Dir(source.LogicalPath))
	result.Nodes = append(result.Nodes, a.node(source, model.NodeSkill, "skill."+name, map[string]model.SafeValue{
		"content":     a.redactor.RedactField("content", string(data)),
		"bytes":       a.redactor.RedactField("bytes", len(data)),
		"code_points": a.redactor.RedactField("code_points", utf8.RuneCount(data)),
	}, "lazy"))
	return result
}

func (a *Adapter) parseJSONObject(result *model.ParsedConfig, source model.ConfigSource, raw map[string]any) {
	for key, value := range raw {
		switch key {
		case "mcpServers":
			servers, _ := value.(map[string]any)
			for name, server := range servers {
				result.Nodes = append(result.Nodes, a.node(source, model.NodeMCPServer, "mcp."+name, a.attributes("mcpServers."+name, server), "always"))
			}
		case "env":
			values, _ := value.(map[string]any)
			for name, envValue := range values {
				result.Nodes = append(result.Nodes, a.node(source, model.NodeEnvironment, "env."+name, map[string]model.SafeValue{
					"value": a.redactor.RedactField("env."+name, envValue),
				}, "environment_present"))
			}
		case "hooks":
			a.parseHooks(result, source, value)
		case "projects":
			// Project state can contain credentials and local MCP configuration.
			// It is parsed only when a dedicated project mapping is implemented.
			continue
		default:
			if _, nested := value.(map[string]any); nested {
				continue
			}
			result.Nodes = append(result.Nodes, a.node(source, model.NodeRule, "setting."+key, map[string]model.SafeValue{
				"value": a.redactor.RedactField(key, value),
			}, "always"))
		}
	}
}

func (a *Adapter) parseHooks(result *model.ParsedConfig, source model.ConfigSource, value any) {
	events, _ := value.(map[string]any)
	for event, groupsValue := range events {
		groups, _ := groupsValue.([]any)
		for groupIndex, groupValue := range groups {
			group, _ := groupValue.(map[string]any)
			hooks, _ := group["hooks"].([]any)
			for hookIndex, hookValue := range hooks {
				name := "hook." + event + "." + itoa(groupIndex) + "." + itoa(hookIndex)
				attributes := a.attributes("hooks."+event, hookValue)
				if matcher, ok := group["matcher"]; ok {
					attributes["matcher"] = a.redactor.RedactField("hooks."+event+".matcher", matcher)
				}
				result.Nodes = append(result.Nodes, a.node(source, model.NodeHook, name, attributes, "event:"+event))
			}
		}
	}
}

func (a *Adapter) parseInstructionTree(root model.ConfigSource) model.ParsedConfig {
	result := model.ParsedConfig{Client: clientID}
	visiting := make(map[string]bool)
	visited := make(map[string]bool)
	var visit func(model.ConfigSource, int, string)
	visit = func(source model.ConfigSource, depth int, parentNodeID string) {
		canonical := source.CanonicalPath
		if canonical == "" {
			canonical = source.LogicalPath
		}
		if visiting[canonical] {
			result.Findings = append(result.Findings, importCycleFinding(source))
			return
		}
		if visited[canonical] {
			return
		}
		if depth > 4 {
			result.Findings = append(result.Findings, importDepthFinding(source))
			return
		}
		data, err := os.ReadFile(source.CanonicalPath)
		if err != nil {
			result.Findings = append(result.Findings, claudeSourceFinding(source, "Imported instruction is not readable"))
			return
		}
		visiting[canonical] = true
		result.Sources = append(result.Sources, source)
		displayName := "instruction." + filepath.Base(source.LogicalPath)
		node := a.node(source, model.NodeInstruction, displayName, map[string]model.SafeValue{
			"content":     a.redactor.RedactField("content", string(data)),
			"bytes":       a.redactor.RedactField("bytes", len(data)),
			"code_points": a.redactor.RedactField("code_points", utf8.RuneCount(data)),
		}, "startup")
		result.Nodes = append(result.Nodes, node)
		if parentNodeID != "" {
			result.Edges = append(result.Edges, model.Edge{
				ID:   model.StableEdgeID(parentNodeID, model.EdgeImports, node.ID),
				From: parentNodeID, To: node.ID, Type: model.EdgeImports,
				Ruleset: rulesetVersion, Evidence: model.EvidenceConfirmed,
			})
		}
		for _, imported := range importsOutsideCode(string(data)) {
			path := imported
			if strings.HasPrefix(path, "~/") {
				path = filepath.Join(a.env.HomeDir, strings.TrimPrefix(path, "~/"))
			} else if !filepath.IsAbs(path) {
				path = filepath.Join(filepath.Dir(source.LogicalPath), path)
			}
			child := a.source(path, source.Scope, model.FormatMarkdown, "instruction", "Claude @path import")
			visit(child, depth+1, node.ID)
		}
		visiting[canonical] = false
		visited[canonical] = true
	}
	visit(root, 0, "")
	return result
}

func importsOutsideCode(text string) []string {
	var imports []string
	inFence := false
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		withoutInline := removeInlineCode(line)
		for _, match := range importPattern.FindAllStringSubmatch(withoutInline, -1) {
			imports = append(imports, strings.TrimRight(match[1], ".,;:)"))
		}
	}
	return imports
}

func removeInlineCode(line string) string {
	parts := strings.Split(line, "`")
	for index := 1; index < len(parts); index += 2 {
		parts[index] = ""
	}
	return strings.Join(parts, "")
}

func (a *Adapter) attributes(prefix string, value any) map[string]model.SafeValue {
	result := make(map[string]model.SafeValue)
	values, ok := value.(map[string]any)
	if !ok {
		result["value"] = a.redactor.RedactField(prefix, value)
		return result
	}
	for key, child := range values {
		switch typed := child.(type) {
		case []any:
			result[key] = a.redactor.RedactField(prefix+"."+key, strings.Join(stringSlice(typed), " "))
		case map[string]any:
			for nestedKey, nestedValue := range typed {
				result[key+"."+nestedKey] = a.redactor.RedactField(prefix+"."+key+"."+nestedKey, nestedValue)
			}
		default:
			result[key] = a.redactor.RedactField(prefix+"."+key, typed)
		}
	}
	return result
}

func (a *Adapter) node(source model.ConfigSource, nodeType model.NodeType, name string, attributes map[string]model.SafeValue, condition string) model.ConfigNode {
	return model.ConfigNode{
		ID: model.StableNodeID(clientID, source.LogicalPath, name), Type: nodeType, Client: clientID,
		DisplayName: name, Attributes: attributes,
		Origins:       []model.Origin{{SourceID: source.ID, LogicalPath: source.LogicalPath, Scope: source.Scope, FieldPath: name, Rule: rulesetVersion}},
		LoadCondition: condition, AdapterConfidence: model.EvidenceConfirmed,
	}
}

func stringSlice(values []any) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, toString(value))
	}
	return result
}

func toString(value any) string {
	data, _ := json.Marshal(value)
	var result string
	if err := json.Unmarshal(data, &result); err == nil {
		return result
	}
	return string(data)
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	var digits [20]byte
	index := len(digits)
	for value > 0 {
		index--
		digits[index] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[index:])
}

func claudeParseFinding(source model.ConfigSource, reason string) model.Finding {
	return claudeFinding("PARSE-0001", model.SeverityHigh, "Configuration could not be parsed", reason, source, "Correct the syntax and scan again.")
}

func claudeSourceFinding(source model.ConfigSource, reason string) model.Finding {
	return claudeFinding("SOURCE-0001", model.SeverityHigh, "Configuration source is unreadable", reason, source, "Restore the source and scan again.")
}

func importCycleFinding(source model.ConfigSource) model.Finding {
	return claudeFinding("SOURCE-0002", model.SeverityHigh, "Instruction import cycle detected", "An @path import reaches a document already being expanded.", source, "Remove one edge from the import cycle.")
}

func importDepthFinding(source model.ConfigSource) model.Finding {
	return claudeFinding("SOURCE-0003", model.SeverityMedium, "Instruction import depth exceeded", "The import chain exceeds four hops.", source, "Flatten the import chain to four hops or fewer.")
}

func claudeFinding(ruleID string, severity model.Severity, summary, reason string, source model.ConfigSource, remediation string) model.Finding {
	return model.Finding{
		RuleID: ruleID, Severity: severity, Evidence: model.EvidenceConfirmed,
		Summary: summary, Reason: reason, Impact: "Some Claude Code context may be missing or ambiguous.",
		AffectedClients: []string{clientID},
		Origins:         []model.Origin{{SourceID: source.ID, LogicalPath: source.LogicalPath, Scope: source.Scope, Rule: rulesetVersion}},
		Remediation:     remediation,
	}
}
