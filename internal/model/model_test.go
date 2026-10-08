package model

import (
	"bytes"
	"strings"
	"testing"
)

func TestStableNodeIDIsDeterministicAndNamespaced(t *testing.T) {
	got := StableNodeID("codex", "project:/repo/AGENTS.md", "instruction.root")
	want := StableNodeID("codex", "project:/repo/AGENTS.md", "instruction.root")

	if got != want {
		t.Fatalf("same identity produced %q and %q", got, want)
	}
	if !strings.HasPrefix(got, "node_") {
		t.Fatalf("ID %q is missing node_ prefix", got)
	}
	if got == StableNodeID("claude", "project:/repo/AGENTS.md", "instruction.root") {
		t.Fatal("client identity did not affect node ID")
	}
}

func TestMarshalCanonicalIgnoresInputOrder(t *testing.T) {
	a := ScanResult{SchemaVersion: "1.0.0", Analysis: Analysis{Findings: []Finding{
		{RuleID: "PATH-0002", Severity: SeverityMedium},
		{RuleID: "PARSE-0001", Severity: SeverityHigh},
	}}}
	b := ScanResult{SchemaVersion: "1.0.0", Analysis: Analysis{Findings: []Finding{
		{RuleID: "PARSE-0001", Severity: SeverityHigh},
		{RuleID: "PATH-0002", Severity: SeverityMedium},
	}}}

	aj, err := MarshalCanonical(a)
	if err != nil {
		t.Fatal(err)
	}
	bj, err := MarshalCanonical(b)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(aj, bj) {
		t.Fatalf("non-deterministic JSON\n%s\n%s", aj, bj)
	}
}
