package main

import (
	"context"
	"os"

	"github.com/Z-lab-boop/harnessscope/internal/cli"
)

func main() {
	os.Exit(cli.Execute(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}
