package snapshots

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Z-lab-boop/harnessscope/internal/model"
)

func fixedClock() time.Time { return time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC) }

func fixtureSafeResult() model.ScanResult {
	return model.ScanResult{SchemaVersion: model.ReportSchemaVersion, Analysis: model.Analysis{
		Clients: []model.ClientResult{{ID: "codex", Effective: model.EffectiveConfig{Client: "codex"}}},
		Sources: []model.ConfigSource{{ID: "source_z", LogicalPath: "./z.toml", Client: "codex"}, {ID: "source_a", LogicalPath: "~/.codex/config.toml", Client: "codex"}},
		Graph:   model.Graph{Nodes: []model.ConfigNode{{ID: "node_z", Client: "codex", Type: model.NodeRule}, {ID: "node_a", Client: "codex", Type: model.NodeRule}}},
	}}
}

func TestStoreRoundTripUsesPrivatePermissions(t *testing.T) {
	root := filepath.Join(t.TempDir(), "snapshots")
	store := NewStore(root, fixedClock)
	metadata, err := store.Save("before-upgrade", fixtureSafeResult())
	if err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]os.FileMode{root: 0o700, metadata.Path: 0o600} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != want {
			t.Fatalf("mode=%o, want %o", info.Mode().Perm(), want)
		}
	}
	if metadata.Name != "before-upgrade" || metadata.SchemaVersion != "1.0.0" || !metadata.CreatedAt.Equal(fixedClock()) {
		t.Fatalf("metadata=%+v", metadata)
	}
	loaded, err := store.Load("before-upgrade")
	if err != nil {
		t.Fatal(err)
	}
	want, _ := model.MarshalCanonical(fixtureSafeResult())
	got, _ := model.MarshalCanonical(loaded)
	if !bytes.Equal(got, want) {
		t.Fatalf("round trip differs:\n%s\n%s", got, want)
	}
	stored, err := os.ReadFile(metadata.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stored, want) {
		t.Fatalf("noncanonical file: %s", stored)
	}
	listed, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(listed, []Metadata{metadata}) {
		t.Fatalf("list=%+v, saved=%+v", listed, metadata)
	}
}

func TestStoreNamesAreAllowlistedAndCannotTraverse(t *testing.T) {
	root := filepath.Join(t.TempDir(), "snapshots")
	store := NewStore(root, fixedClock)
	for _, name := range []string{"", ".", "..", "../outside", "/tmp/outside", "a/b", `a\b`, " leading", "a b", "中文", strings.Repeat("a", 65), "x\x00y"} {
		if _, err := store.Save(name, fixtureSafeResult()); err == nil {
			t.Errorf("Save accepted %q", name)
		}
		if _, err := store.Load(name); err == nil {
			t.Errorf("Load accepted %q", name)
		}
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("invalid names created root: %v", err)
	}
	for _, name := range []string{"A", "0._-", strings.Repeat("a", 64)} {
		if _, err := store.Save(name, fixtureSafeResult()); err != nil {
			t.Fatalf("valid name %q: %v", name, err)
		}
	}
}

func TestStoreListIsSortedAndIgnoresUnrelatedFiles(t *testing.T) {
	root := filepath.Join(t.TempDir(), "snapshots")
	store := NewStore(root, fixedClock)
	listed, err := store.List()
	if err != nil || len(listed) != 0 || listed == nil {
		t.Fatalf("initial list=%v err=%v", listed, err)
	}
	for _, name := range []string{"z-last", "A-first", "middle"} {
		if _, err := store.Save(name, fixtureSafeResult()); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, ".snapshot-temp"), []byte("incomplete"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("other"), 0o600); err != nil {
		t.Fatal(err)
	}
	listed, err = store.List()
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, item := range listed {
		names = append(names, item.Name)
	}
	if !reflect.DeepEqual(names, []string{"A-first", "middle", "z-last"}) {
		t.Fatalf("names=%v", names)
	}
}

func TestStoreRejectsUnknownMajorAndPreservesPreviousSnapshot(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "snapshots"), fixedClock)
	metadata, err := store.Save("baseline", fixtureSafeResult())
	if err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(metadata.Path)
	future := fixtureSafeResult()
	future.SchemaVersion = "2.0.0"
	if _, err := store.Save("baseline", future); err == nil || !strings.Contains(err.Error(), "major schema") {
		t.Fatalf("Save error=%v", err)
	}
	after, _ := os.ReadFile(metadata.Path)
	if !bytes.Equal(before, after) {
		t.Fatal("failed Save changed baseline")
	}
	futureJSON, _ := model.MarshalCanonical(future)
	if err := os.WriteFile(metadata.Path, futureJSON, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load("baseline"); err == nil || !strings.Contains(err.Error(), "major schema") {
		t.Fatalf("Load error=%v", err)
	}
	if _, err := store.List(); err == nil || !strings.Contains(err.Error(), "major schema") {
		t.Fatalf("List error=%v", err)
	}
	future.SchemaVersion = "1.9.7"
	if _, err := store.Save("minor-upgrade", future); err != nil {
		t.Fatalf("compatible minor: %v", err)
	}
}

func TestStoreScrubsCanariesBeforePersistence(t *testing.T) {
	input := fixtureSafeResult()
	canary := "sk-HARNESSSCOPE-CANARY-12345678901234567890"
	input.Analysis.Graph.Nodes[0].Attributes = map[string]model.SafeValue{"token": {Kind: "string", Display: canary, Present: true}}
	input.Analysis.Findings = []model.Finding{{RuleID: "SECRET-0001", Summary: canary}}
	store := NewStore(filepath.Join(t.TempDir(), "snapshots"), fixedClock)
	metadata, err := store.Save("safe", input)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(metadata.Path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("HARNESSSCOPE-CANARY")) || !bytes.Contains(data, []byte("[REDACTED]")) {
		t.Fatalf("unsafe snapshot: %s", data)
	}
	loaded, err := store.Load("safe")
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := model.MarshalCanonical(loaded)
	if bytes.Contains(encoded, []byte(canary)) {
		t.Fatal("loaded canary")
	}
	if input.Analysis.Graph.Nodes[0].Attributes["token"].Display != canary {
		t.Fatal("Save mutated caller input")
	}
}

func TestStoreRefusesSymlinksAndNonregularSnapshots(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "snapshots")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(base, "outside.json")
	original, _ := model.MarshalCanonical(fixtureSafeResult())
	if err := os.WriteFile(outside, original, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape.json")); err != nil {
		t.Fatal(err)
	}
	store := NewStore(root, fixedClock)
	if _, err := store.Load("escape"); err == nil {
		t.Fatal("Load followed symlink")
	}
	if _, err := store.Save("escape", fixtureSafeResult()); err == nil {
		t.Fatal("Save accepted symlink")
	}
	if _, err := store.List(); err == nil {
		t.Fatal("List followed symlink")
	}
	unchanged, _ := os.ReadFile(outside)
	if !bytes.Equal(unchanged, original) {
		t.Fatal("outside file changed")
	}
	if err := os.Mkdir(filepath.Join(root, "directory.json"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load("directory"); err == nil {
		t.Fatal("Load accepted directory")
	}
	link := filepath.Join(base, "linked-root")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	linked := NewStore(link, fixedClock)
	if _, err := linked.Save("new", fixtureSafeResult()); err == nil {
		t.Fatal("Save followed root symlink")
	}
	if _, err := linked.Load("escape"); err == nil {
		t.Fatal("Load followed root symlink")
	}
}

func TestStoreRepairsExistingRootPermissionsAndCleansTemporaryFiles(t *testing.T) {
	root := filepath.Join(t.TempDir(), "snapshots")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	store := NewStore(root, fixedClock)
	if _, err := store.Save("replace", fixtureSafeResult()); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Save("replace", model.ScanResult{}); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(root)
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("mode=%o", info.Mode().Perm())
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 1 || entries[0].Name() != "replace.json" {
		t.Fatalf("entries=%v", entries)
	}
	if _, err := NewStore("", nil).Save("empty", fixtureSafeResult()); err == nil {
		t.Fatal("accepted empty root")
	}
}
