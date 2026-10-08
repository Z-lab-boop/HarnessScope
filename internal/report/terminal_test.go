package report

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Z-lab-boop/harnessscope/internal/model"
)

func TestWriteTerminalIncludesEvidenceAndAction(t *testing.T) {
	result := model.ScanResult{SchemaVersion: model.ReportSchemaVersion, Analysis: model.Analysis{
		Clients:  []model.ClientResult{{ID: "codex", Compatibility: model.CompatibilityMetadata{Tier: model.TierVerified, State: model.CompatibilityVerified}}},
		Findings: []model.Finding{{RuleID: "PATH-0001", Severity: model.SeverityHigh, Evidence: model.EvidenceConfirmed, Summary: "Missing command", Reason: "Path absent", Impact: "Server cannot start", Remediation: "Restore it"}},
	}}
	var output bytes.Buffer

	if err := WriteTerminal(&output, result); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"codex", "VERIFIED", "PATH-0001", "CONFIRMED", "Path absent", "Server cannot start", "Restore it"} {
		if !strings.Contains(output.String(), text) {
			t.Fatalf("terminal output missing %q:\n%s", text, output.String())
		}
	}
}
