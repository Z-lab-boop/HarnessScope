package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Z-lab-boop/harnessscope/internal/model"
	"github.com/Z-lab-boop/harnessscope/internal/report"
)

func newScanCommand(runtime *Runtime, stdout io.Writer) *cobra.Command {
	var clients []string
	var noReport bool
	var failOn string
	var jsonPath string
	var htmlPath string
	var openReport bool
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
			publicResult, err := sanitizeReportPaths(result, runtime.Environment.HomeDir, path)
			if err != nil {
				return err
			}
			if err := report.WriteTerminal(stdout, publicResult); err != nil {
				return err
			}
			if !noReport {
				writtenJSON, writtenHTML, err := writeReports(publicResult, runtime.Environment.AppDataDir, jsonPath, htmlPath)
				if err != nil {
					return err
				}
				fmt.Fprintf(stdout, "JSON report: %s\nHTML report: %s\n", writtenJSON, writtenHTML)
				if openReport {
					if runtime.OpenPath == nil {
						return fmt.Errorf("opening reports is not available")
					}
					if err := runtime.OpenPath(command.Context(), writtenHTML); err != nil {
						return fmt.Errorf("open HTML report: %w", err)
					}
				}
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
	command.Flags().StringVar(&jsonPath, "json", "", "write JSON report to this path")
	command.Flags().StringVar(&htmlPath, "html", "", "write HTML report to this path")
	command.Flags().BoolVar(&openReport, "open", false, "open the generated HTML report")
	return command
}

func sanitizeReportPaths(result model.ScanResult, home, scanRoot string) (model.ScanResult, error) {
	cleanHome := filepath.Clean(home)
	data, err := model.MarshalCanonical(result)
	if err != nil {
		return model.ScanResult{}, fmt.Errorf("prepare public report: %w", err)
	}
	text := string(data)
	if absoluteRoot, absoluteErr := filepath.Abs(scanRoot); absoluteErr == nil && absoluteRoot != string(filepath.Separator) {
		text = strings.ReplaceAll(text, filepath.Clean(absoluteRoot), ".")
	}
	if home != "" && cleanHome != "." && cleanHome != string(filepath.Separator) {
		text = strings.ReplaceAll(text, cleanHome, "~")
	}
	data = []byte(text)
	var sanitized model.ScanResult
	if err := json.Unmarshal(data, &sanitized); err != nil {
		return model.ScanResult{}, fmt.Errorf("sanitize public report paths: %w", err)
	}
	return sanitized, nil
}

func writeReports(result model.ScanResult, appDataDir, requestedJSON, requestedHTML string) (string, string, error) {
	var canonical bytes.Buffer
	if err := report.WriteJSON(&canonical, result); err != nil {
		return "", "", err
	}
	hash := sha256.Sum256(canonical.Bytes())
	id := hex.EncodeToString(hash[:])[:12]
	defaultDir := filepath.Join(appDataDir, "reports", id)
	jsonPath := requestedJSON
	if jsonPath == "" {
		jsonPath = filepath.Join(defaultDir, "report.json")
	}
	htmlPath := requestedHTML
	if htmlPath == "" {
		htmlPath = filepath.Join(defaultDir, "report.html")
	}
	for _, path := range []string{jsonPath, htmlPath} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return "", "", fmt.Errorf("create report directory: %w", err)
		}
	}
	if err := os.WriteFile(jsonPath, canonical.Bytes(), 0o600); err != nil {
		return "", "", fmt.Errorf("write JSON report: %w", err)
	}
	file, err := os.OpenFile(htmlPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return "", "", fmt.Errorf("create HTML report: %w", err)
	}
	writeErr := report.WriteHTML(file, result)
	closeErr := file.Close()
	if writeErr != nil {
		return "", "", fmt.Errorf("write HTML report: %w", writeErr)
	}
	if closeErr != nil {
		return "", "", fmt.Errorf("close HTML report: %w", closeErr)
	}
	return jsonPath, htmlPath, nil
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
