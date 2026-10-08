package resolver

import (
	"fmt"
	"sort"

	"github.com/Z-lab-boop/harnessscope/internal/model"
)

func Build(clientResults []model.ClientResult) model.Graph {
	clients := append([]model.ClientResult(nil), clientResults...)
	sort.Slice(clients, func(i, j int) bool { return clients[i].ID < clients[j].ID })
	nodes := make(map[string]model.ConfigNode)
	edges := make(map[string]model.Edge)
	for _, client := range clients {
		clientNodeID := model.StableNodeID(client.ID, "client", client.ID)
		nodes[clientNodeID] = model.ConfigNode{
			ID: clientNodeID, Type: model.NodeClient, Client: client.ID,
			DisplayName: client.ID, AdapterConfidence: compatibilityEvidence(client.Compatibility.State),
		}
		for _, source := range client.Effective.Sources {
			nodes[source.ID] = model.ConfigNode{
				ID: source.ID, Type: model.NodeSource, Client: client.ID,
				DisplayName: source.LogicalPath, AdapterConfidence: compatibilityEvidence(client.Compatibility.State),
			}
			addEdge(edges, model.Edge{
				From: clientNodeID, To: source.ID, Type: model.EdgeLoads,
				Ruleset:  client.Compatibility.RulesetVersion,
				Evidence: compatibilityEvidence(client.Compatibility.State),
			})
		}
		for _, node := range client.Effective.Nodes {
			nodes[node.ID] = node
			for _, origin := range node.Origins {
				if _, exists := nodes[origin.SourceID]; exists {
					originCopy := origin
					addEdge(edges, model.Edge{
						From: origin.SourceID, To: node.ID, Type: model.EdgeLoads,
						Ruleset:  client.Compatibility.RulesetVersion,
						Evidence: node.AdapterConfidence, Origin: &originCopy,
					})
				}
			}
		}
		for _, edge := range client.Effective.Edges {
			addEdge(edges, edge)
		}
	}

	graph := model.Graph{
		Nodes: make([]model.ConfigNode, 0, len(nodes)),
		Edges: make([]model.Edge, 0, len(edges)),
	}
	for _, node := range nodes {
		graph.Nodes = append(graph.Nodes, node)
	}
	for _, edge := range edges {
		graph.Edges = append(graph.Edges, edge)
	}
	sort.Slice(graph.Nodes, func(i, j int) bool { return graph.Nodes[i].ID < graph.Nodes[j].ID })
	sort.Slice(graph.Edges, func(i, j int) bool { return graph.Edges[i].ID < graph.Edges[j].ID })
	return graph
}

func Validate(graph model.Graph) error {
	nodes := make(map[string]struct{}, len(graph.Nodes))
	for _, node := range graph.Nodes {
		if node.ID == "" {
			return fmt.Errorf("graph contains a node with an empty ID")
		}
		if _, exists := nodes[node.ID]; exists {
			return fmt.Errorf("graph contains duplicate node %q", node.ID)
		}
		nodes[node.ID] = struct{}{}
	}
	for _, edge := range graph.Edges {
		if _, exists := nodes[edge.From]; !exists {
			return fmt.Errorf("edge %q has missing source node %q", edge.ID, edge.From)
		}
		if _, exists := nodes[edge.To]; !exists {
			return fmt.Errorf("edge %q has missing target node %q", edge.ID, edge.To)
		}
	}
	return nil
}

func addEdge(edges map[string]model.Edge, edge model.Edge) {
	if edge.ID == "" {
		edge.ID = model.StableEdgeID(edge.From, edge.Type, edge.To)
	}
	edges[edge.ID] = edge
}

func compatibilityEvidence(state model.CompatibilityState) model.EvidenceStatus {
	if state == model.CompatibilityVerified {
		return model.EvidenceConfirmed
	}
	return model.EvidenceUnknown
}
