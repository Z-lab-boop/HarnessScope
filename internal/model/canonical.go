package model

import (
	"encoding/json"
	"sort"
)

func Canonicalize(input ScanResult) ScanResult {
	result := input
	result.Analysis.Clients = append([]ClientResult(nil), input.Analysis.Clients...)
	result.Analysis.Sources = append([]ConfigSource(nil), input.Analysis.Sources...)
	result.Analysis.Graph.Nodes = append([]ConfigNode(nil), input.Analysis.Graph.Nodes...)
	result.Analysis.Graph.Edges = append([]Edge(nil), input.Analysis.Graph.Edges...)
	result.Analysis.Findings = append([]Finding(nil), input.Analysis.Findings...)
	result.Analysis.Context = append([]ContextEstimate(nil), input.Analysis.Context...)
	result.Analysis.FixPlans = append([]FixPlan(nil), input.Analysis.FixPlans...)

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
