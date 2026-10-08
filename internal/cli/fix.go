package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/Z-lab-boop/harnessscope/internal/fixes"
	"github.com/Z-lab-boop/harnessscope/internal/model"
)

func newFixCommand(runtime *Runtime, stdout io.Writer) *cobra.Command {
	var path string
	var clients []string
	var apply bool
	var dryRun bool
	command := &cobra.Command{
		Use:   "fix [fix-id...]",
		Short: "Preview or apply conservative configuration fixes",
		Args:  cobra.ArbitraryArgs,
		RunE: func(command *cobra.Command, selected []string) error {
			if apply && dryRun {
				return fmt.Errorf("--apply and --dry-run cannot be used together")
			}
			result, err := runtime.Scan(command.Context(), ScanOptions{CWD: path, Clients: clients, NoReport: true})
			if err != nil {
				return err
			}
			plans, err := fixes.Plan(result, selected)
			if err != nil {
				return &ExitError{Code: ExitSafety, Err: err}
			}
			if len(plans) == 0 {
				fmt.Fprintln(stdout, "No SAFE fixes available.")
				return nil
			}
			if !apply {
				fmt.Fprintln(stdout, "HarnessScope fix dry-run (no files changed)")
				return writeFixPlans(stdout, plans)
			}
			backupRoot := filepath.Join(runtime.Environment.AppDataDir, "backups")
			for _, plan := range plans {
				transaction, err := fixes.Apply(command.Context(), plan, backupRoot, verifyFix(runtime, path, clients, plan.ID))
				if err != nil {
					return &ExitError{Code: ExitSafety, Err: fmt.Errorf("apply %s safely: %w", plan.ID, err)}
				}
				fmt.Fprintf(stdout, "Applied %s; backup: %s\n", plan.ID, transaction.BackupID)
			}
			return nil
		},
	}
	command.Flags().StringVar(&path, "path", ".", "workspace path to inspect")
	command.Flags().StringSliceVar(&clients, "client", []string{"all"}, "client to inspect; repeat for multiple clients")
	command.Flags().BoolVar(&apply, "apply", false, "apply SAFE fixes after creating backups")
	command.Flags().BoolVar(&dryRun, "dry-run", false, "preview fixes without changing files (default)")
	return command
}

func writeFixPlans(output io.Writer, plans []model.FixPlan) error {
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	for _, plan := range plans {
		if err := encoder.Encode(plan); err != nil {
			return err
		}
	}
	return nil
}

func verifyFix(runtime *Runtime, path string, clients []string, appliedID string) fixes.VerifyFunc {
	return func(ctx context.Context, _ []string) error {
		result, err := runtime.Scan(ctx, ScanOptions{CWD: path, Clients: clients, NoReport: true})
		if err != nil {
			return err
		}
		remaining, err := fixes.Plan(result, nil)
		if err != nil {
			return err
		}
		for _, plan := range remaining {
			if plan.ID == appliedID {
				return fmt.Errorf("fix %s postcondition is not satisfied", appliedID)
			}
		}
		return nil
	}
}
