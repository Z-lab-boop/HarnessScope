package model

import (
	"encoding/json"
	"sort"
)

func Canonicalize(input ScanResult) ScanResult {
	result := input
	result.Analysis.Clients = cloneSlice(input.Analysis.Clients)
	result.Analysis.Sources = cloneSlice(input.Analysis.Sources)
	result.Analysis.Graph.Nodes = cloneSlice(input.Analysis.Graph.Nodes)
	result.Analysis.Graph.Edges = cloneSlice(input.Analysis.Graph.Edges)
	result.Analysis.Findings = cloneSlice(input.Analysis.Findings)
	result.Analysis.Context = append([]ContextEstimate(nil), input.Analysis.Context...)
	result.Analysis.FixPlans = append([]FixPlan(nil), input.Analysis.FixPlans...)
	for index := range result.Analysis.Clients {
		result.Analysis.Clients[index].Effective = canonicalizeEffective(result.Analysis.Clients[index].Effective)
	}

	sort.Slice(result.Analysis.Clients, func(i, j int) bool {
		return result.Analysis.Clients[i].ID < result.Analysis.Clients[j].ID
	})
	sort.Slice(result.Analysis.Sources, func(i, j int) bool {
		return result.Analysis.Sources[i].ID < result.Analysis.Sources[j].ID
	})
	sort.Slice(result.Analysis.Graph.Nodes, func(i, j int) bool {
		return result.Analysis.Graph.Nodes[i].ID < result.Analysis.Graph.Nodes[j].ID
	})
	sort.Slice(result.Analysis.Graph.Edges, func(i, j int) bool {
		return edgeSortKey(result.Analysis.Graph.Edges[i]) < edgeSortKey(result.Analysis.Graph.Edges[j])
	})
	sort.Slice(result.Analysis.Findings, func(i, j int) bool {
		a, b := result.Analysis.Findings[i], result.Analysis.Findings[j]
		if severityRank(a.Severity) != severityRank(b.Severity) {
			return severityRank(a.Severity) < severityRank(b.Severity)
		}
		if a.RuleID != b.RuleID {
			return a.RuleID < b.RuleID
		}
		return firstOrigin(a) < firstOrigin(b)
	})
	sort.Slice(result.Analysis.Context, func(i, j int) bool {
		a, b := result.Analysis.Context[i], result.Analysis.Context[j]
		if a.Client != b.Client {
			return a.Client < b.Client
		}
		return a.SourceID < b.SourceID
	})
	sort.Slice(result.Analysis.FixPlans, func(i, j int) bool {
		return result.Analysis.FixPlans[i].ID < result.Analysis.FixPlans[j].ID
	})
	return result
}

func canonicalizeEffective(input EffectiveConfig) EffectiveConfig {
	result := input
	result.Sources = cloneSlice(input.Sources)
	result.Nodes = cloneSlice(input.Nodes)
	result.Edges = cloneSlice(input.Edges)
	result.Findings = cloneSlice(input.Findings)
	sort.Slice(result.Sources, func(i, j int) bool { return result.Sources[i].ID < result.Sources[j].ID })
	sort.Slice(result.Nodes, func(i, j int) bool { return result.Nodes[i].ID < result.Nodes[j].ID })
	sort.Slice(result.Edges, func(i, j int) bool { return edgeSortKey(result.Edges[i]) < edgeSortKey(result.Edges[j]) })
	sort.Slice(result.Findings, func(i, j int) bool {
		if result.Findings[i].RuleID != result.Findings[j].RuleID {
			return result.Findings[i].RuleID < result.Findings[j].RuleID
		}
		return firstOrigin(result.Findings[i]) < firstOrigin(result.Findings[j])
	})
	return result
}

func cloneSlice[T any](input []T) []T {
	if len(input) == 0 {
		return []T{}
	}
	return append([]T(nil), input...)
}

func MarshalCanonical(input ScanResult) ([]byte, error) {
	if input.SchemaVersion == "" {
		input.SchemaVersion = ReportSchemaVersion
	}
	data, err := json.MarshalIndent(Canonicalize(input), "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func severityRank(severity Severity) int {
	switch severity {
	case SeverityHigh:
		return 0
	case SeverityMedium:
		return 1
	case SeverityLow:
		return 2
	default:
		return 3
	}
}

func edgeSortKey(edge Edge) string {
	if edge.ID != "" {
		return edge.ID
	}
	return edge.From + "\x00" + string(edge.Type) + "\x00" + edge.To
}

func firstOrigin(finding Finding) string {
	if len(finding.Origins) == 0 {
		return ""
	}
	return finding.Origins[0].LogicalPath + "\x00" + finding.Origins[0].FieldPath
}
