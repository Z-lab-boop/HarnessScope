package cli

import (
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Z-lab-boop/harnessscope/internal/model"
)

func newCompareCommand(runtime *Runtime, stdout io.Writer) *cobra.Command {
	var path string
	command := &cobra.Command{
		Use:   "compare <client> <client> [client...]",
		Short: "Compare normalized configuration across clients",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			result, err := runtime.Scan(command.Context(), ScanOptions{CWD: path, Clients: args, NoReport: true})
			if err != nil {
				return err
			}
			writeComparison(stdout, args, result.Analysis.Graph.Nodes)
			return nil
		},
	}
	command.Flags().StringVar(&path, "path", ".", "working directory to inspect")
	return command
}

func writeComparison(output io.Writer, clients []string, nodes []model.ConfigNode) {
	byName := make(map[string]map[string]model.ConfigNode)
	for _, node := range nodes {
		if node.Type == model.NodeClient || node.Type == model.NodeSource || node.DisplayName == "" {
			continue
		}
		if byName[node.DisplayName] == nil {
			byName[node.DisplayName] = make(map[string]model.ConfigNode)
		}
		if _, exists := byName[node.DisplayName][node.Client]; !exists {
			byName[node.DisplayName][node.Client] = node
		}
	}
	names := make([]string, 0, len(byName))
	for name := range byName {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		entries := byName[name]
		status := comparisonStatus(clients, entries)
		parts := make([]string, 0, len(clients))
		for _, client := range clients {
			if _, exists := entries[client]; exists {
				parts = append(parts, client+":present")
			} else {
				parts = append(parts, client+":missing")
			}
		}
		fmt.Fprintf(output, "%s %s %s\n", name, status, strings.Join(parts, " "))
	}
}

func comparisonStatus(clients []string, entries map[string]model.ConfigNode) string {
	if len(entries) != len(clients) {
		return "missing"
	}
	var first map[string]model.SafeValue
	for _, client := range clients {
		node := entries[client]
		if first == nil {
			first = node.Attributes
			continue
		}
		if !reflect.DeepEqual(first, node.Attributes) {
			return "different"
		}
	}
	return "same"
}
