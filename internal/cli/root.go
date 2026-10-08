package cli

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

const (
	ExitOK          = 0
	ExitThreshold   = 1
	ExitOperational = 2
	ExitSafety      = 3
)

type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string { return e.Err.Error() }
func (e *ExitError) Unwrap() error { return e.Err }

func Execute(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	root := newRootCommand(stdout, stderr)
	root.SetArgs(args)
	if err := root.ExecuteContext(ctx); err != nil {
		var exitErr *ExitError
		if errors.As(err, &exitErr) {
			fmt.Fprintln(stderr, exitErr.Error())
			return exitErr.Code
		}
		fmt.Fprintln(stderr, err)
		return ExitOperational
	}
	return ExitOK
}

func newRootCommand(stdout, stderr io.Writer) *cobra.Command {
	root := &cobra.Command{
		Use:           "hscope",
		Short:         "See what your coding agent actually loads",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetOut(stdout)
	root.SetErr(stderr)
	for _, name := range []string{"scan", "explain", "compare", "report", "fix", "rollback"} {
		commandName := name
		root.AddCommand(&cobra.Command{
			Use:   commandName,
			Short: commandName + " configuration state",
			RunE: func(*cobra.Command, []string) error {
				return fmt.Errorf("%s command not implemented", commandName)
			},
		})
	}
	return root
}
