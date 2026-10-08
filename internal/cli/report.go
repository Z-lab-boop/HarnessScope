package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	hscopereport "github.com/Z-lab-boop/harnessscope/internal/report"
)

func newReportCommand(stdout io.Writer) *cobra.Command {
	var inputPath string
	var htmlPath string
	command := &cobra.Command{
		Use:   "report",
		Short: "Render a saved scan without reading live configuration",
		RunE: func(*cobra.Command, []string) error {
			input, err := os.Open(inputPath)
			if err != nil {
				return fmt.Errorf("open saved report: %w", err)
			}
			result, readErr := hscopereport.ReadJSON(input)
			closeErr := input.Close()
			if readErr != nil {
				return readErr
			}
			if closeErr != nil {
				return closeErr
			}
			if err := os.MkdirAll(filepath.Dir(htmlPath), 0o700); err != nil {
				return err
			}
			output, err := os.OpenFile(htmlPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
			if err != nil {
				return err
			}
			writeErr := hscopereport.WriteHTML(output, result)
			closeErr = output.Close()
			if writeErr != nil {
				return writeErr
			}
			if closeErr != nil {
				return closeErr
			}
			fmt.Fprintln(stdout, htmlPath)
			return nil
		},
	}
	command.Flags().StringVar(&inputPath, "from", "", "saved report JSON")
	command.Flags().StringVar(&htmlPath, "html", "", "output HTML path")
	_ = command.MarkFlagRequired("from")
	_ = command.MarkFlagRequired("html")
	return command
}
