package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Z-lab-boop/harnessscope/internal/model"
)

func TestWriteJSONPreservesDefaultSchemaAndCrossFieldStrings(t *testing.T) {
	var output bytes.Buffer
	result := model.ScanResult{Analysis: model.Analysis{Graph: model.Graph{Nodes: []model.ConfigNode{{ID: "node-a", DisplayName: "https://alice", LoadCondition: "password@example.test"}}}}}
	if err := WriteJSON(&output, result); err != nil {
		t.Fatal(err)
	}
	var decoded model.ScanResult
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.SchemaVersion != model.ReportSchemaVersion || decoded.Analysis.Graph.Nodes[0].DisplayName != "https://alice" || decoded.Analysis.Graph.Nodes[0].LoadCondition != "password@example.test" {
		t.Fatalf("report contract changed: %+v", decoded)
	}
}

func TestWriteJSONIsCanonicalAndSecretSafe(t *testing.T) {
	first := model.ScanResult{SchemaVersion: model.ReportSchemaVersion, Analysis: model.Analysis{Findings: []model.Finding{
		{RuleID: "PATH-0001", Severity: model.SeverityHigh},
		{RuleID: "MCP-0001", Severity: model.SeverityMedium},
	}}}
	second := model.ScanResult{SchemaVersion: model.ReportSchemaVersion, Analysis: model.Analysis{Findings: []model.Finding{
		{RuleID: "MCP-0001", Severity: model.SeverityMedium},
		{RuleID: "PATH-0001", Severity: model.SeverityHigh},
	}}}
	var a, b bytes.Buffer
	if err := WriteJSON(&a, first); err != nil {
		t.Fatal(err)
	}
	if err := WriteJSON(&b, second); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Fatalf("JSON is not canonical\n%s\n%s", a.Bytes(), b.Bytes())
	}
	if strings.Contains(a.String(), "HARNESSSCOPE-CANARY") {
		t.Fatalf("canary leaked: %s", a.String())
	}
}

func TestReadJSONRejectsUnknownMajorSchema(t *testing.T) {
	_, err := ReadJSON(strings.NewReader(`{"schema_version":"2.0.0","analysis":{"clients":[],"sources":[],"graph":{"nodes":[],"edges":[]},"findings":[]}}`))
	if err == nil || !strings.Contains(err.Error(), "major schema") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWriteJSONUsesArraysForRequiredCollections(t *testing.T) {
	result := model.ScanResult{Analysis: model.Analysis{Clients: []model.ClientResult{{
		ID: "cursor", Effective: model.EffectiveConfig{Client: "cursor"},
	}}}}
	var output bytes.Buffer
	if err := WriteJSON(&output, result); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{`"sources": []`, `"nodes": []`, `"edges": []`, `"findings": []`} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("required collection is not an array %s: %s", expected, output.String())
		}
	}
}
