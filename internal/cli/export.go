package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	bundle "github.com/Z-lab-boop/harnessscope/internal/export"
	"github.com/Z-lab-boop/harnessscope/internal/snapshots"
	"github.com/spf13/cobra"
)

func newExportCommand(runtime *Runtime, stdout io.Writer) *cobra.Command {
	var output, baseline string
	var force bool
	command := &cobra.Command{Use: "export [path]", Short: "Export a sanitized diagnostic ZIP (review before public upload)", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if output == "" {
			return fmt.Errorf("--output is required")
		}
		target, err := filepath.Abs(output)
		if err != nil {
			return err
		}
		path := "."
		if len(args) == 1 {
			path = args[0]
		}
		result, store, err := diagnosticScan(cmd.Context(), runtime, path)
		if err != nil {
			return err
		}
		input := bundle.Input{ToolVersion: runtime.ToolVersion, Report: result}
		if baseline != "" {
			previous, err := store.Load(baseline)
			if err != nil {
				return err
			}
			drift := snapshots.Compare(result, previous)
			drift.Baseline = baseline
			input.Drift = &drift
		}
		if err := writeBundleAtomically(target, force, func(writer io.Writer) error {
			_, err := bundle.New(nil).Write(cmd.Context(), writer, input)
			return err
		}); err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdout, target)
		return err
	}}
	command.Flags().StringVar(&output, "output", "", "destination ZIP path (required)")
	command.Flags().StringVar(&baseline, "baseline", "", "include normalized drift against this baseline")
	command.Flags().BoolVar(&force, "force", false, "atomically replace an existing output")
	return command
}

func writeBundleAtomically(target string, force bool, write func(io.Writer) error) (err error) {
	var file *os.File
	if force {
		file, err = os.CreateTemp(filepath.Dir(target), ".hscope-export-*")
	} else {
		file, err = os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	}
	if err != nil {
		return err
	}
	name := file.Name()
	closed, published := false, false
	defer func() {
		if !closed {
			_ = file.Close()
		}
		if !published {
			_ = os.Remove(name)
		}
	}()
	if err = file.Chmod(0o600); err != nil {
		return err
	}
	if err = write(file); err != nil {
		return err
	}
	if err = file.Sync(); err != nil {
		return err
	}
	err = file.Close()
	closed = true
	if err != nil {
		return err
	}
	if force {
		if err = os.Rename(name, target); err != nil {
			return err
		}
	}
	published = true
	return nil
}
