package cli

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/Z-lab-boop/harnessscope/internal/fixes"
)

func newRollbackCommand(runtime *Runtime, stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "rollback <backup-id>",
		Short: "Restore files from one HarnessScope backup",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			backupRoot := filepath.Join(runtime.Environment.AppDataDir, "backups")
			if err := fixes.Rollback(command.Context(), backupRoot, args[0]); err != nil {
				return &ExitError{Code: ExitSafety, Err: fmt.Errorf("rollback safely: %w", err)}
			}
			fmt.Fprintf(stdout, "Restored backup %s\n", args[0])
			return nil
		},
	}
}
