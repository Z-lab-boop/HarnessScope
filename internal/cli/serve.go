package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Z-lab-boop/harnessscope/internal/model"
	"github.com/Z-lab-boop/harnessscope/internal/server"
	"github.com/spf13/cobra"
)

func newServeCommand(runtime *Runtime, stdout, stderr io.Writer) *cobra.Command {
	var clients []string
	var port int
	var open bool
	command := &cobra.Command{
		Use:   "serve [path]",
		Short: "Open the local coding-agent control center",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) (resultErr error) {
			if port < 0 || port > 65535 {
				return fmt.Errorf("dashboard port must be between 0 and 65535")
			}
			ctx, stop := signal.NotifyContext(command.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			if err := ctx.Err(); err != nil {
				return err
			}
			path := "."
			if len(args) == 1 {
				path = args[0]
			}
			workspace, err := filepath.Abs(path)
			if err != nil {
				return fmt.Errorf("resolve workspace: %w", err)
			}
			// Capture roots once: a later working-directory or environment change
			// must not move scans, backups, or snapshots into another workspace.
			captured := *runtime
			captured.Environment.PathEntries = append([]string(nil), runtime.Environment.PathEntries...)
			captured.Environment.HomeDir, err = filepath.Abs(runtime.Environment.HomeDir)
			if err != nil {
				return fmt.Errorf("resolve home directory: %w", err)
			}
			captured.Environment.AppDataDir, err = filepath.Abs(runtime.Environment.AppDataDir)
			if err != nil {
				return fmt.Errorf("resolve app data directory: %w", err)
			}
			selected := append([]string(nil), clients...)
			scan := func(scanCtx context.Context) (model.ScanResult, error) {
				// NewService's initial callback uses Background; combine it with
				// the command lifetime so cancellation also covers startup scans.
				if err := ctx.Err(); err != nil {
					return model.ScanResult{}, err
				}
				requestCtx, cancel := context.WithCancel(scanCtx)
				detach := context.AfterFunc(ctx, cancel)
				defer detach()
				defer cancel()
				result, err := captured.Scan(requestCtx, ScanOptions{CWD: workspace, Clients: selected, NoReport: true})
				if err != nil {
					return model.ScanResult{}, err
				}
				if err := requestCtx.Err(); err != nil {
					return model.ScanResult{}, err
				}
				return result, nil
			}
			service, err := server.NewService(server.ServiceConfig{Workspace: workspace, HomeDir: captured.Environment.HomeDir, AppDataDir: captured.Environment.AppDataDir, Scan: scan, Clock: time.Now})
			if err != nil {
				return err
			}
			start := captured.StartServer
			if start == nil {
				start = startServer
			}
			instance, err := start(ctx, server.HTTPConfig{Port: port, Service: service, Logger: log.New(stderr, "", 0), ToolVersion: captured.ToolVersion})
			if err != nil {
				return err
			}
			defer func() {
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				resultErr = errors.Join(resultErr, instance.Close(shutdownCtx))
			}()
			launchURL := instance.URL()
			if _, err := fmt.Fprintln(stdout, launchURL); err != nil {
				return fmt.Errorf("write dashboard launch URL failed")
			}
			if open {
				if captured.OpenBrowser == nil {
					return fmt.Errorf("opening the dashboard browser is not available")
				}
				// An opener may include its argument in errors. Never echo the
				// launch URL a second time through the diagnostic channel.
				if err := captured.OpenBrowser(ctx, launchURL); err != nil {
					return fmt.Errorf("open dashboard browser failed")
				}
			}
			return instance.Wait()
		},
	}
	command.Flags().StringSliceVar(&clients, "client", []string{"all"}, "client to inspect; repeat for multiple clients")
	command.Flags().IntVar(&port, "port", 0, "loopback port; 0 selects an available port")
	command.Flags().BoolVar(&open, "open", false, "open the dashboard in your browser")
	return command
}
