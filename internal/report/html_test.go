package report

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Z-lab-boop/harnessscope/internal/model"
)

func TestWriteHTMLIsOfflineEscapedAndReadableWithoutJavaScript(t *testing.T) {
	result := model.ScanResult{SchemaVersion: model.ReportSchemaVersion, Analysis: model.Analysis{
		Clients: []model.ClientResult{{ID: "codex", Compatibility: model.CompatibilityMetadata{Tier: model.TierVerified}}},
		Findings: []model.Finding{{
			RuleID: "PATH-0001", Severity: model.SeverityHigh, Evidence: model.EvidenceConfirmed,
			Summary: `<img src=x onerror=alert(1)>`, Reason: "missing", Impact: "cannot start", Remediation: "restore",
		}},
	}}
	var output bytes.Buffer
	if err := WriteHTML(&output, result); err != nil {
		t.Fatal(err)
	}
	html := output.String()
	for _, forbidden := range []string{"http://", "https://", "<script src=", "<link rel="} {
		if strings.Contains(html, forbidden) {
			t.Fatalf("HTML contains external or unsafe reference %q", forbidden)
		}
	}
	if strings.Contains(html, `<img src=x`) || !strings.Contains(html, `&lt;img src=x onerror=alert(1)&gt;`) {
		t.Fatalf("finding was not escaped: %s", html)
	}
	for _, required := range []string{"<table", "PATH-0001", "HIGH", "CONFIRMED", "<svg"} {
		if !strings.Contains(html, required) {
			t.Fatalf("no-JavaScript report missing %q", required)
		}
	}
}
