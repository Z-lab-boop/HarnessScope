package analyzers

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/Z-lab-boop/harnessscope/internal/model"
)

// Options are private scan inputs, never report metadata.
type Options struct {
	PathEntries []string
	ScanRoot    string
	HomeDir     string
}

type optionAnalyzer func(context.Context, model.Analysis, Options) []model.Finding

var advancedRegistry = []optionAnalyzer{
	analyzeCompatibility, analyzeCommands, analyzePortability,
	analyzeDivergence, analyzeSecretPresence, analyzeSymlinkEscape,
}

func analyzeCompatibility(_ context.Context, input model.Analysis, _ Options) []model.Finding {
	var findings []model.Finding
	for _, client := range input.Clients {
		version := client.Detection.Version
		if version == "" {
			version = client.Compatibility.InstalledVersion
		}
		if !client.Detection.Installed || version == "" {
			continue
		}
		verified := false
		for _, candidate := range client.Compatibility.VerifiedVersions {
			if candidate == version {
				verified = true
				break
			}
		}
		if verified {
			continue
		}
		node := model.ConfigNode{ID: model.StableNodeID(client.ID, "client", client.ID), Client: client.ID, AdapterConfidence: model.EvidenceConfirmed}
		findings = append(findings, finding("CLIENT-0001", model.SeverityMedium, behavioralEvidence(input, node),
			"Installed client version is outside the verified set",
			"The adapter has no verification entry for the detected installed version.",
			"Effective behavior may differ from the adapter's verified rules.",
			"Review the compatibility evidence or use a version in the adapter's verified set.", node))
	}
	return findings
}

func analyzeCommands(ctx context.Context, input model.Analysis, options Options) []model.Finding {
	var findings []model.Finding
	for _, node := range input.Graph.Nodes {
		if ctx.Err() != nil {
			break
		}
		if node.Type != model.NodeMCPServer && node.Type != model.NodeHook {
			continue
		}
		value := node.Attributes["command"]
		if !usablePathValue(value) || filepath.IsAbs(value.Display) || strings.ContainsAny(value.Display, `/\`) || len(strings.Fields(value.Display)) != 1 {
			continue
		}
		resolved := false
		for _, directory := range options.PathEntries {
			// Relative search entries are anchored to the fixed scan root when available.
			if !filepath.IsAbs(directory) && options.ScanRoot != "" {
				directory = filepath.Join(options.ScanRoot, directory)
			}
			info, err := os.Stat(filepath.Join(directory, value.Display))
			if err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0 {
				resolved = true
				break
			}
		}
		if resolved {
			continue
		}
		findings = append(findings, finding("CMD-0001", model.SeverityHigh, behavioralEvidence(input, node),
			"Bare command is unavailable on the captured executable search path",
			"No executable file for the configured command was found in the captured search entries.",
			"The configured MCP server or hook may fail to start in this environment.",
			"Install the command or configure an executable available in the client's environment.", node))
	}
	return findings
}

func analyzePortability(ctx context.Context, input model.Analysis, options Options) []model.Finding {
	var findings []model.Finding
	for _, node := range input.Graph.Nodes {
		if ctx.Err() != nil {
			break
		}
		if node.Type != model.NodeMCPServer && node.Type != model.NodeHook {
			continue
		}
		for _, field := range []string{"command", "cwd"} {
			value := node.Attributes[field]
			if !usablePathValue(value) || !filepath.IsAbs(value.Display) {
				continue
			}
			if field == "command" && isHarnessScope(value.Display) {
				continue
			}
			if !withinRoot(value.Display, options.HomeDir) && !withinRoot(value.Display, options.ScanRoot) {
				continue
			}
			if _, err := os.Stat(value.Display); err != nil {
				continue
			}
			findings = append(findings, finding("PATH-0002", model.SeverityLow, behavioralEvidence(input, node),
				"Existing absolute path is tied to this machine",
				"The "+field+" field references an absolute location under the fixed home or workspace root.",
				"The configuration may require changes when shared with another machine or workspace.",
				"Use a portable command or a client-supported relative working directory where possible.", node))
		}
	}
	return findings
}

func analyzeDivergence(_ context.Context, input model.Analysis, _ Options) []model.Finding {
	groups := make(map[string][]model.ConfigNode)
	for _, node := range input.Graph.Nodes {
		if node.Type != model.NodeMCPServer && (node.Type != model.NodeRule || !strings.HasPrefix(node.DisplayName, "setting.")) {
			continue
		}
		key := string(node.Type) + "\x00" + node.DisplayName
		groups[key] = append(groups[key], node)
	}
	var findings []model.Finding
	for _, nodes := range groups {
		if len(nodes) < 2 {
			continue
		}
		sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
		first := displayValues(nodes[0].Attributes)
		divergent := false
		for _, node := range nodes[1:] {
			if !reflect.DeepEqual(first, displayValues(node.Attributes)) {
				divergent = true
				break
			}
		}
		if !divergent {
			continue
		}
		findings = append(findings, finding("CONFIG-0001", model.SeverityMedium, behavioralEvidence(input, nodes...),
			"Normalized configuration declarations have divergent safe values",
			"The same normalized setting or MCP name has different display-safe declarations across sources or clients.",
			"A familiar configuration name may behave differently depending on the client or loaded source.",
			"Compare the origin chains and align the declarations when the divergence is unintended.", nodes...))
	}
	return findings
}

func analyzeSecretPresence(_ context.Context, input model.Analysis, _ Options) []model.Finding {
	var findings []model.Finding
	for _, node := range input.Graph.Nodes {
		var origins []model.Origin
		for _, origin := range node.Origins {
			if origin.Scope == model.ScopeProject || origin.Scope == model.ScopeNested {
				origins = append(origins, origin)
			}
		}
		if len(origins) == 0 {
			continue
		}
		categories := make(map[string]struct{})
		for _, value := range node.Attributes {
			// Presence and category are sufficient; never inspect credential displays.
			if value.Present && value.SecretCategory != "" {
				categories[safeSecretCategory(value.SecretCategory)] = struct{}{}
			}
		}
		if len(categories) == 0 {
			continue
		}
		var names []string
		for category := range categories {
			names = append(names, category)
		}
		sort.Strings(names)
		originNode := node
		originNode.Origins = origins
		findings = append(findings, finding("SECRET-0001", model.SeverityHigh, model.EvidenceConfirmed,
			"Credential-shaped field is present in project configuration",
			"Present credential categories: "+strings.Join(names, ", ")+".",
			"Project configuration may expose credentials when shared or committed.",
			"Move credentials to a supported secret store or environment reference and review repository history.", originNode))
	}
	return findings
}

func analyzeSymlinkEscape(_ context.Context, input model.Analysis, options Options) []model.Finding {
	var findings []model.Finding
	for _, source := range input.Sources {
		if !source.Exists || !source.Symlink || !filepath.IsAbs(source.CanonicalPath) {
			continue
		}
		root := options.ScanRoot
		switch source.Scope {
		case model.ScopeUser, model.ScopeProfile:
			root = options.HomeDir
		case model.ScopeProject, model.ScopeNested, model.ScopeLocal:
		default:
			continue
		}
		if root == "" || withinRoot(source.CanonicalPath, canonicalRoot(root)) {
			continue
		}
		// SOURCE-0003 already identifies Claude import-depth overflow in v0.1.
		node := model.ConfigNode{ID: source.ID, Client: source.Client, Origins: []model.Origin{{SourceID: source.ID, LogicalPath: source.LogicalPath, Scope: source.Scope, Rule: "SOURCE-0004"}}}
		findings = append(findings, finding("SOURCE-0004", model.SeverityMedium, model.EvidenceConfirmed,
			"Configuration symlink resolves outside its declared root",
			"The captured canonical symlink target is outside the fixed workspace or user configuration root.",
			"The source may load configuration from an unexpected location outside the scan boundary.",
			"Review the symlink target and keep it within the source's declared configuration root.", node))
	}
	return findings
}

func behavioralEvidence(input model.Analysis, nodes ...model.ConfigNode) model.EvidenceStatus {
	result := model.EvidenceConfirmed
	for _, node := range nodes {
		switch node.AdapterConfidence {
		case model.EvidenceConfirmed:
		case model.EvidenceLikely:
			result = model.EvidenceLikely
		default:
			return model.EvidenceUnknown
		}
		for _, client := range input.Clients {
			if client.ID == node.Client && (client.Compatibility.Tier == model.TierPreview || client.Compatibility.State != model.CompatibilityVerified) {
				return model.EvidenceUnknown
			}
		}
	}
	return result
}

func usablePathValue(value model.SafeValue) bool {
	return value.Present && value.Display != "" && value.Display != "[REDACTED]" && value.SecretCategory == ""
}

func withinRoot(path, root string) bool {
	if !filepath.IsAbs(path) || !filepath.IsAbs(root) {
		return false
	}
	relative, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func canonicalRoot(root string) string {
	if canonical, err := filepath.EvalSymlinks(root); err == nil {
		return canonical
	}
	return root
}

func isHarnessScope(path string) bool {
	name := strings.ToLower(filepath.Base(path))
	if name == "harnessscope" || name == "harnessscope.exe" {
		return true
	}
	executable, err := os.Executable()
	return err == nil && filepath.Clean(path) == filepath.Clean(executable)
}

func displayValues(attributes map[string]model.SafeValue) map[string]model.SafeValue {
	result := make(map[string]model.SafeValue)
	for field, value := range attributes {
		if value.Present {
			result[field] = model.SafeValue{Display: value.Display, Present: true}
		}
	}
	return result
}

func safeSecretCategory(category string) string {
	switch category {
	case "credential_field", "private_key", "openai_key", "github_token", "authorization", "url_credentials", "high_entropy_literal":
		return category
	default:
		return "credential"
	}
}
