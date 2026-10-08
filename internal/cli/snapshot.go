package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"

	"github.com/Z-lab-boop/harnessscope/internal/model"
	"github.com/Z-lab-boop/harnessscope/internal/sanitize"
	"github.com/Z-lab-boop/harnessscope/internal/snapshots"
	"github.com/spf13/cobra"
)

// Resolve and capture roots before scanning. An invalid application-data root
// must never silently redirect diagnostics into the workspace.
func diagnosticScan(ctx context.Context, runtime *Runtime, path string) (model.ScanResult, *snapshots.Store, error) {
	if !filepath.IsAbs(runtime.Environment.AppDataDir) {
		return model.ScanResult{}, nil, fmt.Errorf("application data directory must be absolute")
	}
	appData := filepath.Clean(runtime.Environment.AppDataDir)
	workspace, err := filepath.Abs(path)
	if err != nil {
		return model.ScanResult{}, nil, err
	}
	home, err := filepath.Abs(runtime.Environment.HomeDir)
	if err != nil {
		return model.ScanResult{}, nil, err
	}
	if err := ctx.Err(); err != nil {
		return model.ScanResult{}, nil, err
	}
	result, err := runtime.Scan(ctx, ScanOptions{CWD: workspace, Clients: []string{"all"}, NoReport: true})
	if err != nil {
		return model.ScanResult{}, nil, err
	}
	if err := ctx.Err(); err != nil {
		return model.ScanResult{}, nil, err
	}
	public, err := sanitize.Report(result, home, workspace)
	return public, snapshots.NewStore(filepath.Join(appData, "snapshots"), nil), err
}

func newSnapshotCommand(runtime *Runtime, stdout io.Writer) *cobra.Command {
	command := &cobra.Command{Use: "snapshot", Short: "Save and compare sanitized local baselines"}
	save := &cobra.Command{Use: "save <name> [path]", Short: "Save or replace a baseline in application data", Args: cobra.RangeArgs(1, 2), RunE: func(cmd *cobra.Command, args []string) error {
		path := "."
		if len(args) == 2 {
			path = args[1]
		}
		result, store, err := diagnosticScan(cmd.Context(), runtime, path)
		if err != nil {
			return err
		}
		meta, err := store.Save(args[0], result)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdout, meta.Path)
		return err
	}}
	list := &cobra.Command{Use: "list", Short: "List saved baselines as JSON", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		_, store, err := diagnosticScan(cmd.Context(), runtime, ".")
		if err != nil {
			return err
		}
		items, err := store.List()
		if err != nil {
			return err
		}
		return json.NewEncoder(stdout).Encode(items)
	}}
	diff := &cobra.Command{Use: "diff <name> [path]", Short: "Compare normalized state with a baseline as JSON", Args: cobra.RangeArgs(1, 2), RunE: func(cmd *cobra.Command, args []string) error {
		path := "."
		if len(args) == 2 {
			path = args[1]
		}
		result, store, err := diagnosticScan(cmd.Context(), runtime, path)
		if err != nil {
			return err
		}
		baseline, err := store.Load(args[0])
		if err != nil {
			return err
		}
		drift := snapshots.Compare(result, baseline)
		drift.Baseline = args[0]
		return json.NewEncoder(stdout).Encode(drift)
	}}
	command.AddCommand(save, list, diff)
	return command
}
