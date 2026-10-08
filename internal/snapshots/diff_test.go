package snapshots

import (
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Z-lab-boop/harnessscope/internal/model"
)

func driftFixture() model.ScanResult {
	return model.ScanResult{SchemaVersion: model.ReportSchemaVersion, Analysis: model.Analysis{
		Clients:  []model.ClientResult{{ID: "codex", Detection: model.DetectionResult{Installed: true, Version: "1.0.0"}, Capabilities: model.AdapterCapabilities{Instructions: true}, Compatibility: model.CompatibilityMetadata{Tier: model.TierVerified, State: model.CompatibilityVerified, InstalledVersion: "1.0.0", VerifiedVersions: []string{"1.0.0", "0.9.0"}, RulesetVersion: "codex-2026-10-08", SupportedFeatures: []string{"skills", "instructions"}}}},
		Sources:  []model.ConfigSource{{ID: "source_a", Client: "codex", Scope: model.ScopeProject, Format: model.FormatTOML, Exists: true, Readable: true, Kind: "config"}},
		Graph:    model.Graph{Nodes: []model.ConfigNode{{ID: "node_a", Type: model.NodeRule, Client: "codex", AdapterConfidence: model.EvidenceConfirmed, Origins: []model.Origin{{SourceID: "source_a", FieldPath: "model", Scope: model.ScopeProject}}}}},
		Findings: []model.Finding{{RuleID: "MCP-0001", Severity: model.SeverityMedium, Evidence: model.EvidenceConfirmed, AffectedClients: []string{"codex"}, GraphReferences: []string{"node_a"}, Origins: []model.Origin{{SourceID: "source_a", FieldPath: "mcp.servers", Scope: model.ScopeProject}}}},
	}}
}

func TestCompareDetectsAddedRemovedAndChangedEntities(t *testing.T) {
	empty := model.ScanResult{SchemaVersion: model.ReportSchemaVersion}
	baseline := driftFixture()
	changed := driftFixture()
	changed.Analysis.Clients[0].Capabilities.Hooks = true
	changed.Analysis.Clients[0].Compatibility.State = model.CompatibilityUnknown
	changed.Analysis.Sources[0].Readable = false
	changed.Analysis.Graph.Nodes[0].AdapterConfidence = model.EvidenceUnknown
	changed.Analysis.Findings[0].Severity = model.SeverityHigh
	for _, tc := range []struct {
		name, kind        string
		current, baseline model.ScanResult
	}{
		{"added", "ADDED", baseline, empty}, {"removed", "REMOVED", empty, baseline}, {"changed", "CHANGED", changed, baseline},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := Compare(tc.current, tc.baseline)
			if got.SchemaVersion != "1.0.0" {
				t.Fatalf("schema=%s", got.SchemaVersion)
			}
			wantTypes := []string{"CLIENT", "COMPATIBILITY", "FINDING", "NODE", "SOURCE"}
			if len(got.Changes) != len(wantTypes) {
				t.Fatalf("changes=%+v", got.Changes)
			}
			for i, change := range got.Changes {
				if change.EntityType != wantTypes[i] || change.Kind != tc.kind || change.ID == "" || change.Summary == "" || !reflect.DeepEqual(change.Clients, []string{"codex"}) {
					t.Fatalf("change=%+v", change)
				}
				wantID := map[string]string{"CLIENT": "codex", "COMPATIBILITY": "codex", "NODE": "node_a", "SOURCE": "source_a"}[change.EntityType]
				if wantID != "" && change.ID != wantID {
					t.Fatalf("identifier=%s, want %s", change.ID, wantID)
				}
				if change.EntityType == "FINDING" && !strings.HasPrefix(change.ID, "MCP-0001:") {
					t.Fatalf("finding identifier=%s", change.ID)
				}
			}
		})
	}
}

func TestCompareIsDeterministicAndValueSafe(t *testing.T) {
	baseline := driftFixture()
	current := driftFixture()
	for _, input := range []*model.ScanResult{&baseline, &current} {
		input.Analysis.Clients = append(input.Analysis.Clients, model.ClientResult{ID: "claude"})
		input.Analysis.Sources = append(input.Analysis.Sources, model.ConfigSource{ID: "source_b", Client: "claude"})
		input.Analysis.Graph.Nodes = append(input.Analysis.Graph.Nodes, model.ConfigNode{ID: "node_b", Client: "claude", Type: model.NodeSkill})
		input.Analysis.Findings = append(input.Analysis.Findings, model.Finding{RuleID: "PATH-0001", AffectedClients: []string{"claude", "codex"}, GraphReferences: []string{"node_a", "node_b"}, Origins: []model.Origin{{SourceID: "source_b"}, {SourceID: "source_a"}}})
	}
	current.Analysis.Graph.Nodes[0].Type = model.NodeHook
	current.Analysis.Graph.Nodes[0].DisplayName = "HARNESSSCOPE-CANARY"
	current.Analysis.Graph.Nodes[0].Attributes = map[string]model.SafeValue{"content": {Kind: "string", Display: "HARNESSSCOPE-CANARY", Present: true}}
	current.Analysis.Findings[0].Summary = "HARNESSSCOPE-CANARY"
	current.Analysis.Sources[0].LogicalPath = "/private/HARNESSSCOPE-CANARY/config.toml"
	current.Analysis.Findings[1].Severity = model.SeverityHigh
	beforeCurrent, _ := json.Marshal(current)
	beforeBaseline, _ := json.Marshal(baseline)
	first := Compare(current, baseline)
	afterCurrent, _ := json.Marshal(current)
	afterBaseline, _ := json.Marshal(baseline)
	if string(beforeCurrent) != string(afterCurrent) || string(beforeBaseline) != string(afterBaseline) {
		t.Fatal("Compare mutated its input")
	}
	for _, input := range []*model.ScanResult{&baseline, &current} {
		slices.Reverse(input.Analysis.Clients)
		slices.Reverse(input.Analysis.Sources)
		slices.Reverse(input.Analysis.Graph.Nodes)
		slices.Reverse(input.Analysis.Findings)
		for i := range input.Analysis.Clients {
			slices.Reverse(input.Analysis.Clients[i].Compatibility.VerifiedVersions)
			slices.Reverse(input.Analysis.Clients[i].Compatibility.SupportedFeatures)
		}
		for i := range input.Analysis.Findings {
			slices.Reverse(input.Analysis.Findings[i].AffectedClients)
			slices.Reverse(input.Analysis.Findings[i].Origins)
			slices.Reverse(input.Analysis.Findings[i].GraphReferences)
		}
	}
	second := Compare(current, baseline)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("non-deterministic:\n%+v\n%+v", first, second)
	}
	encoded, _ := json.Marshal(first)
	if strings.Contains(string(encoded), "HARNESSSCOPE-CANARY") || strings.Contains(string(encoded), "/private/") {
		t.Fatalf("unsafe drift: %s", encoded)
	}
	if len(first.Changes) != 2 {
		t.Fatalf("unexpected changes=%+v", first.Changes)
	}
}

func TestCompareIgnoresValuesPathsTimesAndFreeText(t *testing.T) {
	baseline, current := driftFixture(), driftFixture()
	baseline.RunMetadata = &model.RunMetadata{GeneratedAt: "2026-10-08", CWD: "/home/before"}
	current.RunMetadata = &model.RunMetadata{GeneratedAt: "2026-10-09", CWD: "/home/after"}
	current.Analysis.Clients[0].Detection.Executable = "/private/bin/codex"
	current.Analysis.Clients[0].Detection.Error = "HARNESSSCOPE-CANARY"
	current.Analysis.Clients[0].Compatibility.LastVerificationAt = "2099-01-01"
	current.Analysis.Clients[0].Compatibility.EvidenceReferences = []string{"/private/evidence"}
	current.Analysis.Sources[0].LogicalPath = "/private/config.toml"
	current.Analysis.Sources[0].CanonicalPath = "/private/canonical/config.toml"
	current.Analysis.Sources[0].DiscoveryReason = "HARNESSSCOPE-CANARY"
	current.Analysis.Graph.Nodes[0].DisplayName = "HARNESSSCOPE-CANARY"
	current.Analysis.Graph.Nodes[0].LoadCondition = "HARNESSSCOPE-CANARY"
	current.Analysis.Graph.Nodes[0].Attributes = map[string]model.SafeValue{"HARNESSSCOPE-CANARY": {Kind: "string", Display: "HARNESSSCOPE-CANARY", Present: true}}
	current.Analysis.Graph.Nodes[0].Origins[0].LogicalPath = "/private/origin"
	current.Analysis.Graph.Nodes[0].Origins[0].Line = 42
	current.Analysis.Graph.Nodes[0].Origins[0].Rule = "HARNESSSCOPE-CANARY"
	current.Analysis.Findings[0].Summary = "HARNESSSCOPE-CANARY"
	current.Analysis.Findings[0].Reason = "HARNESSSCOPE-CANARY"
	current.Analysis.Findings[0].Impact = "HARNESSSCOPE-CANARY"
	current.Analysis.Findings[0].Remediation = "HARNESSSCOPE-CANARY"
	current.Analysis.Findings[0].FixPlanID = "HARNESSSCOPE-CANARY"
	current.Analysis.Findings[0].Origins[0].LogicalPath = "/private/finding"
	got := Compare(current, baseline)
	if len(got.Changes) != 0 || got.Changes == nil {
		t.Fatalf("private fields caused drift: %+v", got)
	}
	encoded, _ := json.Marshal(got)
	if string(encoded) != `{"schema_version":"1.0.0","changes":[]}` {
		t.Fatalf("empty diff=%s", encoded)
	}
}

func TestCompareNormalizesFieldsAndRetainsRepeatedRuleLocations(t *testing.T) {
	baseline, current := driftFixture(), driftFixture()
	current.Analysis.Clients[0].ID = " codex "
	current.Analysis.Clients[0].Compatibility.SupportedFeatures = []string{" instructions ", "skills", "skills"}
	current.Analysis.Findings[0].AffectedClients = []string{"codex", " codex "}
	if got := Compare(current, baseline); len(got.Changes) != 0 {
		t.Fatalf("normalization drift=%+v", got)
	}
	second := baseline.Analysis.Findings[0]
	second.GraphReferences = []string{"node_b"}
	second.Origins = []model.Origin{{SourceID: "source_b", FieldPath: "mcp.servers"}}
	baseline.Analysis.Findings = append(baseline.Analysis.Findings, second)
	current = driftFixture()
	current.Analysis.Findings[0].Severity = model.SeverityHigh
	got := Compare(current, baseline)
	if len(got.Changes) != 2 {
		t.Fatalf("collapsed repeated rule: %+v", got)
	}
	kinds := []string{got.Changes[0].Kind, got.Changes[1].Kind}
	slices.Sort(kinds)
	if !reflect.DeepEqual(kinds, []string{"CHANGED", "REMOVED"}) || got.Changes[0].ID == got.Changes[1].ID {
		t.Fatalf("repeated rule changes=%+v", got)
	}
}

func TestCompareRejectsUnsafeIdentifierContents(t *testing.T) {
	baseline, current := driftFixture(), driftFixture()
	for _, input := range []*model.ScanResult{&baseline, &current} {
		input.Analysis.Sources[0].ID = "/private/HARNESSSCOPE-CANARY/baseline"
		input.Analysis.Graph.Nodes[0].ID = "sk-HARNESSSCOPE-CANARY-12345678901234567890"
		input.Analysis.Findings[0].GraphReferences = []string{"/private/HARNESSSCOPE-CANARY/node"}
		input.Analysis.Clients[0].Compatibility.InstalledVersion = "/private/HARNESSSCOPE-CANARY/version"
	}
	current.Analysis.Sources[0].Readable = false
	current.Analysis.Graph.Nodes[0].Type = model.NodeHook
	current.Analysis.Findings[0].Severity = model.SeverityHigh
	current.Analysis.Clients[0].Compatibility.State = model.CompatibilityUnknown
	encoded, _ := json.Marshal(Compare(current, baseline))
	if strings.Contains(string(encoded), "HARNESSSCOPE-CANARY") || strings.Contains(string(encoded), "/private/") {
		t.Fatalf("unsafe identifiers=%s", encoded)
	}
	current.Analysis.Sources[0].ID = "/other/private/path"
	if got := Compare(current, baseline); len(got.Changes) != 4 {
		t.Fatalf("absolute path affected identity=%+v", got)
	}
}

func TestCompareHandlesDuplicateEntityIDsDeterministically(t *testing.T) {
	baseline, current := driftFixture(), driftFixture()
	duplicate := current.Analysis.Graph.Nodes[0]
	duplicate.Type = model.NodeHook
	current.Analysis.Graph.Nodes = append(current.Analysis.Graph.Nodes, duplicate)
	first := Compare(current, baseline)
	slices.Reverse(current.Analysis.Graph.Nodes)
	second := Compare(current, baseline)
	if !reflect.DeepEqual(first, second) || len(first.Changes) != 1 || first.Changes[0].Kind != "CHANGED" {
		t.Fatalf("duplicate comparison=%+v / %+v", first, second)
	}
}

func TestDriftSchemaIsStrictAndMatchesPublicContract(t *testing.T) {
	data, err := os.ReadFile("../../schemas/drift-v1.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	if schema["$schema"] != "https://json-schema.org/draft/2020-12/schema" || schema["additionalProperties"] != false {
		t.Fatalf("non-strict schema=%v", schema)
	}
	change := schema["$defs"].(map[string]any)["change"].(map[string]any)
	if change["additionalProperties"] != false {
		t.Fatal("change allows unknown fields")
	}
	properties := change["properties"].(map[string]any)
	for key, want := range map[string][]string{"kind": {"ADDED", "REMOVED", "CHANGED"}, "entity_type": {"CLIENT", "SOURCE", "NODE", "FINDING", "COMPATIBILITY"}} {
		encoded, _ := json.Marshal(properties[key].(map[string]any)["enum"])
		expected, _ := json.Marshal(want)
		if string(encoded) != string(expected) {
			t.Fatalf("%s enum=%s", key, encoded)
		}
	}
}
