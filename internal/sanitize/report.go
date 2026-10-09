// Package sanitize provides the shared path boundary for public reports.
package sanitize

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Z-lab-boop/harnessscope/internal/model"
	"github.com/Z-lab-boop/harnessscope/internal/secrets"
)

// Report returns a canonical deep copy with workspace and home paths collapsed.
func Report(input model.ScanResult, homeDir, workspaceRoot string) (model.ScanResult, error) {
	if workspaceRoot == "" {
		return model.ScanResult{}, fmt.Errorf("public report workspace root is empty")
	}
	root, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return model.ScanResult{}, fmt.Errorf("resolve public report workspace root: %w", err)
	}
	root = filepath.Clean(root)
	if root == string(filepath.Separator) {
		return model.ScanResult{}, fmt.Errorf("public report workspace root is the filesystem root")
	}
	if input.SchemaVersion == "" {
		input.SchemaVersion = model.ReportSchemaVersion
	}
	home := filepath.Clean(homeDir)
	data, err := secrets.MapJSONStrings(model.Canonicalize(input), func(text string) string {
		text = strings.ReplaceAll(text, root, ".")
		if homeDir != "" && home != "." && home != string(filepath.Separator) {
			text = strings.ReplaceAll(text, home, "~")
		}
		return text
	})
	if err != nil {
		return model.ScanResult{}, fmt.Errorf("sanitize public report strings: %w", err)
	}
	var output model.ScanResult
	if err := json.Unmarshal(data, &output); err != nil {
		return model.ScanResult{}, fmt.Errorf("decode safe report: %w", err)
	}
	return model.Canonicalize(output), nil
}
