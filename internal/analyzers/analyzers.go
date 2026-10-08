package analyzers

import (
	"context"
	"sort"
	"strings"

	"github.com/Z-lab-boop/harnessscope/internal/model"
)

type Analyzer func(context.Context, model.Analysis) []model.Finding

var registry = []Analyzer{
	analyzeDuplicateMCP,
	analyzePaths,
	analyzeDuplicateContext,
}

func Run(ctx context.Context, input model.Analysis) []model.Finding {
	return RunWithOptions(ctx, input, Options{})
}

func RunWithOptions(ctx context.Context, input model.Analysis, options Options) []model.Finding {
	var findings []model.Finding
	for _, analyzer := range registry {
		findings = append(findings, analyzer(ctx, input)...)
	}
	for _, analyzer := range advancedRegistry {
		findings = append(findings, analyzer(ctx, input, options)...)
	}
	sortFindings(findings)
	return findings
}

func sortFindings(findings []model.Finding) {
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].RuleID != findings[j].RuleID {
			return findings[i].RuleID < findings[j].RuleID
		}
		if findings[i].Summary != findings[j].Summary {
			return findings[i].Summary < findings[j].Summary
		}
		return strings.Join(findings[i].GraphReferences, "\x00") < strings.Join(findings[j].GraphReferences, "\x00")
	})
}

func finding(ruleID string, severity model.Severity, evidence model.EvidenceStatus, summary, reason, impact, remediation string, nodes ...model.ConfigNode) model.Finding {
	result := model.Finding{
		RuleID: ruleID, Severity: severity, Evidence: evidence,
		Summary: summary, Reason: reason, Impact: impact, Remediation: remediation,
	}
	clients := make(map[string]struct{})
	for _, node := range nodes {
		if node.Client != "" {
			clients[node.Client] = struct{}{}
		}
		result.GraphReferences = append(result.GraphReferences, node.ID)
		result.Origins = append(result.Origins, node.Origins...)
	}
	for client := range clients {
		result.AffectedClients = append(result.AffectedClients, client)
	}
	sort.Strings(result.AffectedClients)
	sort.Strings(result.GraphReferences)
	return result
}

func weakestEvidence(nodes []model.ConfigNode) model.EvidenceStatus {
	result := model.EvidenceConfirmed
	for _, node := range nodes {
		if node.AdapterConfidence == model.EvidenceUnknown {
			return model.EvidenceUnknown
		}
		if node.AdapterConfidence == model.EvidenceLikely {
			result = model.EvidenceLikely
		}
	}
	return result
}
