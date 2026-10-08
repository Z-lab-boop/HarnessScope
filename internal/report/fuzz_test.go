package report

import (
	"bytes"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/Z-lab-boop/harnessscope/internal/model"
)

func FuzzReportWritersEscapeHostileText(f *testing.F) {
	for _, seed := range []string{"plain", "<script>alert(1)</script>", "&quot;", "\x00\xff", "中文"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, hostile string) {
		result := model.ScanResult{SchemaVersion: model.ReportSchemaVersion, Analysis: model.Analysis{Findings: []model.Finding{{
			RuleID: "FUZZ-0001", Severity: model.SeverityInfo, Evidence: model.EvidenceUnknown,
			Summary: hostile, Reason: hostile, Impact: hostile, Remediation: hostile,
		}}}}
		var jsonOutput, htmlOutput bytes.Buffer
		if err := WriteJSON(&jsonOutput, result); err != nil {
			t.Fatal(err)
		}
		if err := WriteHTML(&htmlOutput, result); err != nil {
			t.Fatal(err)
		}
		if !utf8.Valid(jsonOutput.Bytes()) || !utf8.Valid(htmlOutput.Bytes()) {
			t.Fatal("report output is not valid UTF-8")
		}
		if strings.Contains(hostile, "<script>") && strings.Contains(htmlOutput.String(), hostile) {
			t.Fatal("hostile HTML propagated without escaping")
		}
	})
}

func FuzzCanonicalJSONIgnoresFindingOrder(f *testing.F) {
	f.Add("A", "B")
	f.Fuzz(func(t *testing.T, firstID, secondID string) {
		first := model.Finding{RuleID: firstID, Severity: model.SeverityLow}
		second := model.Finding{RuleID: secondID, Severity: model.SeverityHigh}
		a := model.ScanResult{Analysis: model.Analysis{Findings: []model.Finding{first, second}}}
		b := model.ScanResult{Analysis: model.Analysis{Findings: []model.Finding{second, first}}}
		var left, right bytes.Buffer
		if err := WriteJSON(&left, a); err != nil {
			t.Fatal(err)
		}
		if err := WriteJSON(&right, b); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(left.Bytes(), right.Bytes()) {
			t.Fatal("equivalent finding order produced different JSON")
		}
	})
}
