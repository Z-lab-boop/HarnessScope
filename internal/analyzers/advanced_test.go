package analyzers

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Z-lab-boop/harnessscope/internal/model"
)

func TestAdvancedCompatibility(t *testing.T) {
	for _, tc := range []struct {
		name      string
		installed bool
		version   string
		verified  []string
		want      bool
	}{
		{"outside verified set", true, "2.0", []string{"1.0"}, true},
		{"verified version", true, "1.0", []string{"1.0"}, false},
		{"not installed", false, "2.0", []string{"1.0"}, false},
		{"version unavailable", true, "", []string{"1.0"}, false},
		{"no verified versions", true, "2.0", nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := model.Analysis{Clients: []model.ClientResult{{
				ID: "cursor", Detection: model.DetectionResult{Installed: tc.installed, Version: tc.version},
				Compatibility: model.CompatibilityMetadata{Tier: model.TierPreview, State: model.CompatibilityUnknown, VerifiedVersions: tc.verified},
			}}}
			got := RunWithOptions(context.Background(), input, Options{})
			if tc.want {
				assertAdvancedFinding(t, got, "CLIENT-0001", model.SeverityMedium, model.EvidenceUnknown, []string{"cursor"}, nil)
			} else {
				assertNoAdvancedRule(t, got, "CLIENT-0001")
			}
		})
	}
}

func TestAdvancedCommandsUseOnlyCapturedPATHWithoutExecution(t *testing.T) {
	bin := t.TempDir()
	marker := filepath.Join(bin, "executed")
	tool := filepath.Join(bin, "available-tool")
	if err := os.WriteFile(tool, []byte("#!/bin/sh\ntouch '"+marker+"'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "not-executable"), []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(bin, "directory-tool"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	for _, tc := range []struct {
		name, command string
		entries       []string
		want          bool
	}{
		{"missing", "missing-tool", []string{bin}, true},
		{"executable", "available-tool", []string{bin}, false},
		{"ambient PATH ignored", "available-tool", []string{t.TempDir()}, true},
		{"non executable", "not-executable", []string{bin}, true},
		{"directory", "directory-tool", []string{bin}, true},
		{"absolute handled by PATH rules", tool, []string{bin}, false},
		{"relative command is not bare", "./available-tool", []string{bin}, false},
		{"shell expression is not bare", "available-tool --flag", []string{bin}, false},
		{"redacted", "[REDACTED]", []string{bin}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n := advancedNode("codex", model.NodeMCPServer, "mcp.shared", model.EvidenceConfirmed, "command", tc.command)
			got := RunWithOptions(context.Background(), advancedAnalysis(n), Options{PathEntries: tc.entries})
			if tc.want {
				assertAdvancedFinding(t, got, "CMD-0001", model.SeverityHigh, model.EvidenceConfirmed, []string{"codex"}, n.Origins)
			} else {
				assertNoAdvancedRule(t, got, "CMD-0001")
			}
		})
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("configured command executed: %v", err)
	}
}

func TestAdvancedPortability(t *testing.T) {
	home, workspace, outside := t.TempDir(), t.TempDir(), t.TempDir()
	tool := filepath.Join(home, "tool")
	self := filepath.Join(home, "harnessscope")
	for _, path := range []string{tool, self} {
		if err := os.WriteFile(path, []byte("fixture"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name, field, value string
		want               bool
	}{
		{"home command", "command", tool, true},
		{"workspace directory", "cwd", workspace, true},
		{"outside fixed roots", "cwd", outside, false},
		{"missing path", "command", filepath.Join(home, "missing"), false},
		{"self executable", "command", self, false},
		{"self named directory", "cwd", filepath.Dir(self), true},
		{"relative", "cwd", "./work", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n := advancedNode("cursor", model.NodeHook, "hook.event", model.EvidenceUnknown, tc.field, tc.value)
			got := RunWithOptions(context.Background(), advancedAnalysis(n), Options{ScanRoot: workspace, HomeDir: home})
			if tc.want {
				assertAdvancedFinding(t, got, "PATH-0002", model.SeverityLow, model.EvidenceUnknown, []string{"cursor"}, n.Origins)
			} else {
				assertNoAdvancedRule(t, got, "PATH-0002")
			}
		})
	}
}

func TestAdvancedDivergence(t *testing.T) {
	for _, typ := range []model.NodeType{model.NodeMCPServer, model.NodeRule} {
		for _, tc := range []struct {
			name, right string
			want        bool
		}{{"different", "other-tool", true}, {"same", "first-tool", false}} {
			t.Run(string(typ)+"/"+tc.name, func(t *testing.T) {
				name, field := "mcp.shared", "command"
				if typ == model.NodeRule {
					name, field = "setting.theme", "value"
				}
				a := advancedNode("codex", typ, name, model.EvidenceConfirmed, field, "first-tool")
				b := advancedNode("cursor", typ, name, model.EvidenceUnknown, field, tc.right)
				got := RunWithOptions(context.Background(), advancedAnalysis(a, b), Options{})
				if tc.want {
					assertAdvancedFinding(t, got, "CONFIG-0001", model.SeverityMedium, model.EvidenceUnknown, []string{"codex", "cursor"}, append(a.Origins, b.Origins...))
				} else {
					assertNoAdvancedRule(t, got, "CONFIG-0001")
				}
			})
		}
	}
	// Distinct credentials are equally redacted and must not be compared as raw values.
	a := advancedNode("codex", model.NodeRule, "setting.token", model.EvidenceConfirmed, "value", "[REDACTED]")
	b := advancedNode("claude", model.NodeRule, "setting.token", model.EvidenceConfirmed, "value", "[REDACTED]")
	a.Attributes["value"] = model.SafeValue{Present: true, Display: "[REDACTED]", SecretCategory: "credential_field"}
	b.Attributes["value"] = model.SafeValue{Present: true, Display: "[REDACTED]", SecretCategory: "high_entropy_literal"}
	assertNoAdvancedRule(t, RunWithOptions(context.Background(), advancedAnalysis(a, b), Options{}), "CONFIG-0001")
}

func TestAdvancedSecretPresenceUsesCategoryAndProjectOrigin(t *testing.T) {
	for _, tc := range []struct {
		name, category string
		scope          model.Scope
		present, want  bool
	}{
		{"project credential", "credential_field", model.ScopeProject, true, true},
		{"nested credential", "url_credentials", model.ScopeNested, true, true},
		{"user credential", "credential_field", model.ScopeUser, true, false},
		{"ordinary project value", "", model.ScopeProject, true, false},
		{"absent credential", "credential_field", model.ScopeProject, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n := advancedNode("cursor", model.NodeRule, "setting.auth", model.EvidenceUnknown, "value", "[REDACTED]")
			n.Origins[0].Scope = tc.scope
			n.Attributes["value"] = model.SafeValue{Display: "[REDACTED]", Present: tc.present, SecretCategory: tc.category}
			got := RunWithOptions(context.Background(), advancedAnalysis(n), Options{})
			if tc.want {
				assertAdvancedFinding(t, got, "SECRET-0001", model.SeverityHigh, model.EvidenceConfirmed, []string{"cursor"}, n.Origins)
			} else {
				assertNoAdvancedRule(t, got, "SECRET-0001")
			}
		})
	}
}

func TestAdvancedSymlinkEscapePreservesExistingSourceRule(t *testing.T) {
	root, home, outside := t.TempDir(), t.TempDir(), t.TempDir()
	// ConfigSource.CanonicalPath is already symlink-resolved by discovery.
	for _, path := range []*string{&root, &home, &outside} {
		canonical, err := filepath.EvalSymlinks(*path)
		if err != nil {
			t.Fatal(err)
		}
		*path = canonical
	}
	for _, tc := range []struct {
		name, logical, canonical string
		scope                    model.Scope
		symlink, want            bool
	}{
		{"project escape", filepath.Join(root, "config"), filepath.Join(outside, "config"), model.ScopeProject, true, true},
		{"project inside", filepath.Join(root, "config"), filepath.Join(root, "nested", "config"), model.ScopeProject, true, false},
		{"project cannot escape into home", filepath.Join(root, "config"), filepath.Join(home, "config"), model.ScopeProject, true, true},
		{"user inside home", filepath.Join(home, ".codex", "config"), filepath.Join(home, "shared", "config"), model.ScopeUser, true, false},
		{"user escape", filepath.Join(home, ".codex", "config"), filepath.Join(outside, "config"), model.ScopeUser, true, true},
		{"same prefix outside", filepath.Join(root, "config"), root + "-sibling/config", model.ScopeProject, true, true},
		{"regular source", filepath.Join(root, "config"), filepath.Join(outside, "config"), model.ScopeProject, false, false},
		{"unresolved symlink", filepath.Join(root, "config"), "", model.ScopeProject, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := model.ConfigSource{ID: "source", Client: "cursor", LogicalPath: tc.logical, CanonicalPath: tc.canonical, Scope: tc.scope, Exists: true, Symlink: tc.symlink}
			input := model.Analysis{Sources: []model.ConfigSource{source}}
			got := RunWithOptions(context.Background(), input, Options{ScanRoot: root, HomeDir: home})
			if tc.want {
				assertAdvancedFinding(t, got, "SOURCE-0004", model.SeverityMedium, model.EvidenceConfirmed, []string{"cursor"}, []model.Origin{{SourceID: "source", LogicalPath: tc.logical, Scope: tc.scope, Rule: "SOURCE-0004"}})
			} else {
				assertNoAdvancedRule(t, got, "SOURCE-0004")
			}
			assertNoAdvancedRule(t, got, "SOURCE-0003")
		})
	}
}

func TestAdvancedRulesPreserveWeakestEvidence(t *testing.T) {
	for _, evidence := range []model.EvidenceStatus{model.EvidenceLikely, model.EvidenceUnknown} {
		t.Run(string(evidence), func(t *testing.T) {
			a := advancedNode("codex", model.NodeMCPServer, "mcp.shared", model.EvidenceConfirmed, "command", "missing-tool")
			b := advancedNode("cursor", model.NodeMCPServer, "mcp.shared", evidence, "command", "other-tool")
			got := RunWithOptions(context.Background(), advancedAnalysis(a, b), Options{PathEntries: []string{t.TempDir()}})
			assertAdvancedFinding(t, got, "CONFIG-0001", model.SeverityMedium, evidence, []string{"codex", "cursor"}, append(a.Origins, b.Origins...))
			for _, f := range got {
				if f.RuleID == "CMD-0001" && reflect.DeepEqual(f.AffectedClients, []string{"cursor"}) && f.Evidence != evidence {
					t.Fatalf("weaker command evidence promoted: %#v", f)
				}
			}
		})
	}
	// Even an accidentally CONFIRMED node cannot promote a preview client's behavior.
	n := advancedNode("cursor", model.NodeHook, "hook.event", model.EvidenceConfirmed, "command", "missing-tool")
	input := advancedAnalysis(n)
	input.Clients = []model.ClientResult{{ID: "cursor", Compatibility: model.CompatibilityMetadata{Tier: model.TierPreview, State: model.CompatibilityUnknown}}}
	assertAdvancedFinding(t, RunWithOptions(context.Background(), input, Options{}), "CMD-0001", model.SeverityHigh, model.EvidenceUnknown, []string{"cursor"}, n.Origins)
}

func TestAdvancedDoesNotPublishPATHEntries(t *testing.T) {
	entry := t.TempDir()
	n := advancedNode("codex", model.NodeMCPServer, "mcp.shared", model.EvidenceConfirmed, "command", "missing-tool")
	got := RunWithOptions(context.Background(), advancedAnalysis(n), Options{PathEntries: []string{entry}})
	data, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), entry) {
		t.Fatal("captured PATH entry leaked into findings")
	}
}

func advancedNode(client string, typ model.NodeType, name string, evidence model.EvidenceStatus, field, display string) model.ConfigNode {
	return model.ConfigNode{ID: client + "-" + name, Client: client, Type: typ, DisplayName: name, AdapterConfidence: evidence,
		Attributes: map[string]model.SafeValue{field: {Kind: "string", Display: display, Present: true}},
		Origins:    []model.Origin{{SourceID: "source-" + client, LogicalPath: "./" + client + ".json", FieldPath: name, Scope: model.ScopeProject, Rule: "fixture"}},
	}
}

func advancedAnalysis(nodes ...model.ConfigNode) model.Analysis {
	return model.Analysis{Graph: model.Graph{Nodes: nodes}}
}

func assertAdvancedFinding(t *testing.T, got []model.Finding, rule string, severity model.Severity, evidence model.EvidenceStatus, clients []string, origins []model.Origin) {
	t.Helper()
	for _, f := range got {
		if f.RuleID != rule {
			continue
		}
		if f.Severity != severity || f.Evidence != evidence || !reflect.DeepEqual(f.AffectedClients, clients) || !reflect.DeepEqual(f.Origins, origins) {
			t.Fatalf("unexpected %s: %#v; want severity=%s evidence=%s clients=%v origins=%v", rule, f, severity, evidence, clients, origins)
		}
		if f.Summary == "" || f.Reason == "" || f.Impact == "" || f.Remediation == "" {
			t.Fatalf("incomplete finding: %#v", f)
		}
		return
	}
	t.Fatalf("%s missing: %#v", rule, got)
}

func assertNoAdvancedRule(t *testing.T, got []model.Finding, rule string) {
	t.Helper()
	if containsRule(got, rule) {
		t.Fatalf("unexpected %s: %#v", rule, got)
	}
}
