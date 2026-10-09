package sanitize

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Z-lab-boop/harnessscope/internal/model"
)

func TestReportCollapsesEveryPublicPathWithoutMutatingInput(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, "work")
	fields := []struct {
		name string
		set  func(*model.ScanResult, string)
		get  func(model.ScanResult) string
	}{
		{"source logical", func(r *model.ScanResult, p string) { r.Analysis.Sources[0].LogicalPath = p }, func(r model.ScanResult) string { return r.Analysis.Sources[0].LogicalPath }},
		{"source canonical", func(r *model.ScanResult, p string) { r.Analysis.Sources[0].CanonicalPath = p }, func(r model.ScanResult) string { return r.Analysis.Sources[0].CanonicalPath }},
		{"origin", func(r *model.ScanResult, p string) { r.Analysis.Graph.Nodes[0].Origins[0].LogicalPath = p }, func(r model.ScanResult) string { return r.Analysis.Graph.Nodes[0].Origins[0].LogicalPath }},
		{"graph label", func(r *model.ScanResult, p string) { r.Analysis.Graph.Nodes[0].DisplayName = p }, func(r model.ScanResult) string { return r.Analysis.Graph.Nodes[0].DisplayName }},
		{"executable", func(r *model.ScanResult, p string) { r.Analysis.Clients[0].Detection.Executable = p }, func(r model.ScanResult) string { return r.Analysis.Clients[0].Detection.Executable }},
		{"run metadata", func(r *model.ScanResult, p string) { r.RunMetadata.CWD = p }, func(r model.ScanResult) string { return r.RunMetadata.CWD }},
		{"fix target", func(r *model.ScanResult, p string) { r.Analysis.FixPlans[0].Edits[0].TargetPath = p }, func(r model.ScanResult) string { return r.Analysis.FixPlans[0].Edits[0].TargetPath }},
		{"replacement", func(r *model.ScanResult, p string) { r.Analysis.FixPlans[0].Edits[0].Replacement = p }, func(r model.ScanResult) string { return r.Analysis.FixPlans[0].Edits[0].Replacement }},
		{"safe value", func(r *model.ScanResult, p string) {
			r.Analysis.Graph.Nodes[0].Attributes["path"] = model.SafeValue{Kind: "path", Display: p, Present: true}
		}, func(r model.ScanResult) string { return r.Analysis.Graph.Nodes[0].Attributes["path"].Display }},
	}
	for _, field := range fields {
		for _, prefix := range []struct{ name, path, want string }{
			{"home", home, "~/item"},
			{"workspace", root, "./item"},
		} {
			t.Run(field.name+"/"+prefix.name, func(t *testing.T) {
				input := fixtureResultWithPaths(home, root)
				field.set(&input, filepath.Join(prefix.path, "item"))
				before := canonical(t, input)
				got, err := Report(input, home, root)
				if err != nil {
					t.Fatal(err)
				}
				encoded := canonical(t, got)
				if bytes.Contains(encoded, []byte(home)) || bytes.Contains(encoded, []byte(root)) {
					t.Fatalf("absolute path leaked: %s", encoded)
				}
				if value := field.get(got); value != prefix.want {
					t.Fatalf("path = %q, want %q", value, prefix.want)
				}
				if !bytes.Contains(encoded, []byte("~/")) || !bytes.Contains(encoded, []byte("./")) {
					t.Fatalf("home/workspace markers missing: %s", encoded)
				}
				if !bytes.Equal(before, canonical(t, input)) {
					t.Fatal("input mutated")
				}
				field.set(&got, "changed output")
				if !bytes.Equal(before, canonical(t, input)) {
					t.Fatal("output aliases input")
				}
			})
		}
	}
}

func TestReportSanitizesJSONEscapedRootsInValuesAndKeys(t *testing.T) {
	home := filepath.Join(t.TempDir(), "R&D<private>")
	root := filepath.Join(home, "work&bench")
	path := filepath.Join(root, "config.json")
	input := fixtureResultWithPaths(home, root)
	input.Analysis.Graph.Nodes[0].Attributes = map[string]model.SafeValue{
		path: {Kind: "string", Display: path, Present: true},
	}
	got, err := Report(input, home, root)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encoded, []byte(home)) || bytes.Contains(encoded, []byte(root)) || strings.Contains(string(encoded), `R\u0026D`) {
		t.Fatalf("escaped root leaked: %s", encoded)
	}
	value, ok := got.Analysis.Graph.Nodes[0].Attributes["./config.json"]
	if !ok || value.Display != "./config.json" {
		t.Fatalf("decoded strings were not sanitized: %+v", got.Analysis.Graph.Nodes[0].Attributes)
	}
}

func TestReportRejectsUnsafeWorkspaceRoots(t *testing.T) {
	for _, root := range []string{"", string(filepath.Separator), "/tmp/.."} {
		t.Run(root, func(t *testing.T) {
			if _, err := Report(model.ScanResult{}, t.TempDir(), root); err == nil {
				t.Fatal("unsafe workspace root accepted")
			}
		})
	}
}

func TestReportSkipsUnsafeHomePrefixes(t *testing.T) {
	root := t.TempDir()
	for _, home := range []string{"", ".", string(filepath.Separator)} {
		t.Run(home, func(t *testing.T) {
			input := model.ScanResult{RunMetadata: &model.RunMetadata{CWD: "/outside/a.b"}}
			got, err := Report(input, home, root)
			if err != nil {
				t.Fatal(err)
			}
			if got.RunMetadata.CWD != "/outside/a.b" {
				t.Fatalf("unsafe home corrupted path: %q", got.RunMetadata.CWD)
			}
			if got.SchemaVersion != model.ReportSchemaVersion {
				t.Fatalf("schema default lost: %q", got.SchemaVersion)
			}
		})
	}
}

func fixtureResultWithPaths(home, root string) model.ScanResult {
	return model.ScanResult{
		Analysis: model.Analysis{
			Sources: []model.ConfigSource{{ID: "source", LogicalPath: filepath.Join(home, "config"), CanonicalPath: filepath.Join(root, "config")}},
			Clients: []model.ClientResult{{ID: "codex", Detection: model.DetectionResult{Executable: filepath.Join(home, "bin", "codex")}}},
			Graph: model.Graph{Nodes: []model.ConfigNode{{
				ID: "node", DisplayName: filepath.Join(root, "label"),
				Origins:    []model.Origin{{LogicalPath: filepath.Join(home, "origin")}},
				Attributes: map[string]model.SafeValue{"path": {Kind: "path", Display: filepath.Join(root, "safe"), Present: true}},
			}}},
			FixPlans: []model.FixPlan{{ID: "fix", Edits: []model.Edit{{TargetPath: filepath.Join(root, "target"), Replacement: filepath.Join(home, "replacement")}}}},
		},
		RunMetadata: &model.RunMetadata{CWD: root},
	}
}

func canonical(t *testing.T, result model.ScanResult) []byte {
	t.Helper()
	data, err := model.MarshalCanonical(result)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
