// This process hosts only the synthetic workspace passed by the browser test.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Z-lab-boop/harnessscope/internal/model"
	"github.com/Z-lab-boop/harnessscope/internal/server"
)

func main() {
	root := os.Args[1]
	path := filepath.Join(root, "AGENTS.md")
	s, err := server.NewService(server.ServiceConfig{Workspace: root, HomeDir: root, AppDataDir: filepath.Join(root, "data"), Scan: func(context.Context) (model.ScanResult, error) {
		return model.ScanResult{Analysis: model.Analysis{Sources: []model.ConfigSource{{ID: "instructions", CanonicalPath: path, LogicalPath: path, Exists: true, Readable: true}}}}, nil
	}})
	if err != nil {
		panic(err)
	}
	instance, err := server.Start(context.Background(), server.HTTPConfig{Service: s})
	if err != nil {
		panic(err)
	}
	fmt.Println(instance.URL())
	_, _ = io.Copy(io.Discard, os.Stdin)
	_ = instance.Close(context.Background())
}
