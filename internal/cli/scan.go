package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Z-lab-boop/harnessscope/internal/model"
	"github.com/Z-lab-boop/harnessscope/internal/report"
)

func newScanCommand(runtime *Runtime, stdout io.Writer) *cobra.Command {
	var clients []string
	var noReport bool
	var failOn string
	command := &cobra.Command{
		Use:   "scan [path]",
		Short: "Inspect effective coding-agent configuration",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			path := "."
			if len(args) == 1 {
				path = args[0]
			}
			threshold, err := parseThreshold(failOn)
			if err != nil {
				return err
			}
			result, err := runtime.Scan(command.Context(), ScanOptions{CWD: path, Clients: clients, NoReport: noReport, FailOn: threshold})
			if err != nil {
				return err
			}
			if err := report.WriteTerminal(stdout, result); err != nil {
				return err
			}
			if crossesThreshold(result.Analysis.Findings, threshold) {
				return &ExitError{Code: ExitThreshold, Err: fmt.Errorf("finding threshold %s crossed", strings.ToLower(string(threshold)))}
			}
			return nil
		},
	}
	command.Flags().StringSliceVar(&clients, "client", []string{"all"}, "client to inspect; repeat for multiple clients")
	command.Flags().BoolVar(&noReport, "no-report", false, "do not write JSON or HTML reports")
	command.Flags().StringVar(&failOn, "fail-on", "none", "return exit 1 at high, medium, low, or info")
	return command
}

func parseThreshold(value string) (model.Severity, error) {
	switch strings.ToLower(value) {
	case "", "none":
		return "", nil
	case "high":
		return model.SeverityHigh, nil
	case "medium":
		return model.SeverityMedium, nil
	case "low":
		return model.SeverityLow, nil
	case "info":
		return model.SeverityInfo, nil
	default:
		return "", fmt.Errorf("invalid --fail-on value %q", value)
	}
}

func crossesThreshold(findings []model.Finding, threshold model.Severity) bool {
	if threshold == "" {
		return false
	}
	for _, finding := range findings {
		if severityRank(finding.Severity) <= severityRank(threshold) {
			return true
		}
	}
	return false
}

func severityRank(severity model.Severity) int {
	switch severity {
	case model.SeverityHigh:
		return 0
	case model.SeverityMedium:
		return 1
	case model.SeverityLow:
		return 2
	default:
		return 3
	}
}
