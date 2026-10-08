package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/Z-lab-boop/harnessscope/internal/model"
)

func newExplainCommand(runtime *Runtime, stdout io.Writer) *cobra.Command {
	var client string
	var path string
	command := &cobra.Command{
		Use:   "explain <element>",
		Short: "Explain one configuration element's provenance",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			result, err := runtime.Scan(command.Context(), ScanOptions{CWD: path, Clients: []string{client}, NoReport: true})
			if err != nil {
				return err
			}
			node := findElement(result.Analysis.Graph.Nodes, args[0], client)
			if node == nil {
				return fmt.Errorf("configuration element %q was not found for %s", args[0], client)
			}
			fmt.Fprintf(stdout, "%s (%s) [%s]\n", node.DisplayName, node.Type, node.AdapterConfidence)
			fmt.Fprintf(stdout, "load condition: %s\n", node.LoadCondition)
			for _, origin := range node.Origins {
				fmt.Fprintf(stdout, "origin: %s field=%s scope=%s rule=%s\n", origin.LogicalPath, origin.FieldPath, origin.Scope, origin.Rule)
			}
			for _, edge := range result.Analysis.Graph.Edges {
				if edge.From == node.ID || edge.To == node.ID {
					fmt.Fprintf(stdout, "edge: %s %s -> %s [%s]\n", edge.Type, edge.From, edge.To, edge.Evidence)
				}
			}
			return nil
		},
	}
	command.Flags().StringVar(&client, "client", "codex", "client whose element should be explained")
	command.Flags().StringVar(&path, "path", ".", "working directory to inspect")
	return command
}

func findElement(nodes []model.ConfigNode, query, client string) *model.ConfigNode {
	for index := range nodes {
		if nodes[index].Client == client && (nodes[index].ID == query || nodes[index].DisplayName == query) {
			return &nodes[index]
		}
	}
	return nil
}
