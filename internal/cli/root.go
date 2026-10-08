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
	runtime, err := DefaultRuntime()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return ExitOperational
	}
	return ExecuteWithRuntime(ctx, args, stdout, stderr, runtime)
}

func ExecuteWithRuntime(ctx context.Context, args []string, stdout, stderr io.Writer, runtime *Runtime) int {
	root := newRootCommand(runtime, stdout, stderr)
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

func newRootCommand(runtime *Runtime, stdout, stderr io.Writer) *cobra.Command {
	root := &cobra.Command{
		Use:           "hscope",
		Version:       runtime.ToolVersion,
		Short:         "See what your coding agent actually loads",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.AddCommand(
		newScanCommand(runtime, stdout),
		newExplainCommand(runtime, stdout),
		newCompareCommand(runtime, stdout),
		newReportCommand(stdout),
		newFixCommand(runtime, stdout),
		newRollbackCommand(runtime, stdout),
		newServeCommand(runtime, stdout, stderr),
		newSnapshotCommand(runtime, stdout),
		newExportCommand(runtime, stdout),
	)
	return root
}
