package report

import (
	"fmt"
	"io"
	"sort"

	"github.com/Z-lab-boop/harnessscope/internal/model"
)

func WriteTerminal(output io.Writer, result model.ScanResult) error {
	if _, err := fmt.Fprintln(output, "HarnessScope"); err != nil {
		return err
	}
	for _, client := range result.Analysis.Clients {
		installed := "not installed"
		if client.Detection.Installed {
			installed = client.Detection.Version
		}
		if _, err := fmt.Fprintf(output, "- %s: %s %s (%s)\n", client.ID, client.Compatibility.Tier, client.Compatibility.State, installed); err != nil {
			return err
		}
	}
	counts := make(map[model.Severity]int)
	for _, finding := range result.Analysis.Findings {
		counts[finding.Severity]++
	}
	if _, err := fmt.Fprintf(output, "Findings: HIGH=%d MEDIUM=%d LOW=%d INFO=%d\n", counts[model.SeverityHigh], counts[model.SeverityMedium], counts[model.SeverityLow], counts[model.SeverityInfo]); err != nil {
		return err
	}
	findings := append([]model.Finding(nil), result.Analysis.Findings...)
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Severity != findings[j].Severity {
			return findings[i].Severity < findings[j].Severity
		}
		return findings[i].RuleID < findings[j].RuleID
	})
	for _, finding := range findings {
		if _, err := fmt.Fprintf(output, "\n[%s] %s %s (%s)\nreason: %s\nimpact: %s\naction: %s\n", finding.Severity, finding.RuleID, finding.Summary, finding.Evidence, finding.Reason, finding.Impact, finding.Remediation); err != nil {
			return err
		}
		for _, origin := range finding.Origins {
			if _, err := fmt.Fprintf(output, "source: %s %s\n", origin.LogicalPath, origin.FieldPath); err != nil {
				return err
			}
		}
	}
	return nil
}
