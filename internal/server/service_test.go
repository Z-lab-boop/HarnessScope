package server

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Z-lab-boop/harnessscope/internal/model"
)

func TestServiceApplyRollbackOrderRestoresOverlappingTransactions(t *testing.T) {
	root := t.TempDir()
	script := filepath.Join(root, "hook.sh")
	noncanonical := root + "/./hook.sh"
	original := []byte("#!/bin/sh\n# " + noncanonical + "\n")
	if err := os.WriteFile(script, original, 0o640); err != nil {
		t.Fatal(err)
	}
	raw := model.ScanResult{Analysis: model.Analysis{
		Sources: []model.ConfigSource{{ID: "script", CanonicalPath: script, LogicalPath: script, Exists: true, Readable: true}},
		Graph: model.Graph{Nodes: []model.ConfigNode{
			{ID: "hook", Type: model.NodeHook, Attributes: map[string]model.SafeValue{"command": {Display: script, Present: true}}},
			{ID: "path", Attributes: map[string]model.SafeValue{"path": {Display: noncanonical, Present: true}}, Origins: []model.Origin{{SourceID: "script"}}},
		}},
	}}
	calls := 0
	s, err := NewService(ServiceConfig{Workspace: root, HomeDir: filepath.Dir(root), AppDataDir: filepath.Join(root, "data"), Scan: func(context.Context) (model.ScanResult, error) {
		calls++
		if calls == 4 {
			// Both overlapping transactions have been applied. Fail publication
			// after verification by making backup-history listing unavailable.
			if err := os.Rename(filepath.Join(root, "data", "backups"), filepath.Join(root, "data", "saved")); err != nil {
				return model.ScanResult{}, err
			}
			if err := os.Symlink(filepath.Join(root, "data", "saved"), filepath.Join(root, "data", "backups")); err != nil {
				return model.ScanResult{}, err
			}
		}
		return raw, nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	before := s.State()
	if len(before.FixPlans) != 2 || !strings.HasPrefix(before.FixPlans[0].ID, "FIX-HOOK-") || !strings.HasPrefix(before.FixPlans[1].ID, "FIX-PATH-") {
		t.Fatalf("overlap fixture: %+v", before.FixPlans)
	}
	if _, err := s.ApplyFixes(context.Background(), 1, []string{before.FixPlans[0].ID, before.FixPlans[1].ID}); err == nil {
		t.Fatal("publication failure accepted")
	}
	data, _ := os.ReadFile(script)
	info, _ := os.Stat(script)
	// Forward rollback would leave mode 0751 from the PATH backup. Reverse
	// rollback restores the original 0640 mode from the first HOOK transaction.
	if !bytes.Equal(data, original) || info.Mode().Perm() != 0o640 {
		t.Fatalf("overlapping rollback order failed: mode=%o bytes=%s", info.Mode().Perm(), data)
	}
	if !reflect.DeepEqual(before, s.State()) {
		t.Fatal("failed publication changed state")
	}
}

func TestServiceVerificationRejectsStillPresentFix(t *testing.T) {
	var paths []string
	s, paths, _ := serviceFixture(t, 1, func(n int) error {
		if n == 3 {
			return os.WriteFile(paths[0], []byte("keep\nkeep\n"), 0o640)
		}
		return nil
	})
	if _, err := s.ApplyFixes(context.Background(), 1, []string{s.State().FixPlans[0].ID}); err == nil || !strings.Contains(err.Error(), "postcondition") {
		t.Fatalf("accepted unsatisfied fix: %v", err)
	}
	if s.State().Revision != 1 {
		t.Fatal("verification failure published")
	}
}

func TestServiceRollbackScanFailurePreservesPublishedState(t *testing.T) {
	fail := atomic.Bool{}
	s, paths, _ := serviceFixture(t, 1, func(int) error {
		if fail.Load() {
			return errors.New("scan failed")
		}
		return nil
	})
	applied, err := s.ApplyFixes(context.Background(), 1, []string{s.State().FixPlans[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	fail.Store(true)
	if _, err := s.Rollback(context.Background(), 2, applied.Backups[0].ID); err == nil {
		t.Fatal("failed scan accepted")
	}
	if !reflect.DeepEqual(applied, s.State()) {
		t.Fatal("failed rollback scan published state")
	}
	data, _ := os.ReadFile(paths[0])
	if string(data) != "keep\nkeep\n" {
		t.Fatal("rollback did not occur before scan")
	}
}

func TestServiceRejectsNonSAFESelection(t *testing.T) {
	s, paths, calls := serviceFixture(t, 1, nil)
	s.mu.Lock()
	s.public.FixPlans[0].Risk = model.RiskReview
	s.mu.Unlock()
	if _, err := s.ApplyFixes(context.Background(), 1, []string{s.State().FixPlans[0].ID}); err == nil {
		t.Fatal("REVIEW plan accepted")
	}
	if calls.Load() != 1 {
		t.Fatal("non-SAFE selection scanned")
	}
	data, _ := os.ReadFile(paths[0])
	if string(data) != "keep\nkeep\n" {
		t.Fatal("non-SAFE selection wrote")
	}
}

func TestServiceCanceledOperationsHaveNoEffects(t *testing.T) {
	s, _, calls := serviceFixture(t, 1, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var output bytes.Buffer
	operations := []func() error{
		func() error { _, e := s.Rescan(ctx, 1); return e },
		func() error { _, e := s.PlanFixes(ctx, 1, nil); return e },
		func() error { _, e := s.ApplyFixes(ctx, 1, []string{s.State().FixPlans[0].ID}); return e },
		func() error { _, e := s.Rollback(ctx, 1, "bad"); return e },
		func() error { _, e := s.SaveSnapshot(ctx, 1, "baseline"); return e },
		func() error { _, e := s.CompareSnapshot(ctx, 1, "baseline"); return e },
		func() error { _, e := s.Export(ctx, 1, &output, "0.2.0"); return e },
	}
	for _, op := range operations {
		if err := op(); !errors.Is(err, context.Canceled) {
			t.Fatalf("got %v", err)
		}
	}
	if calls.Load() != 1 || output.Len() != 0 {
		t.Fatal("canceled operation had effects")
	}
	if _, err := os.Stat(s.config.AppDataDir); !os.IsNotExist(err) {
		t.Fatal("canceled operation touched store")
	}
}

func TestServiceInvalidConfiguration(t *testing.T) {
	root := t.TempDir()
	valid := ServiceConfig{Workspace: filepath.Join(root, "workspace"), HomeDir: root, AppDataDir: filepath.Join(root, "data"), Scan: func(context.Context) (model.ScanResult, error) { return model.ScanResult{}, nil }}
	for _, change := range []func(*ServiceConfig){func(c *ServiceConfig) { c.Scan = nil }, func(c *ServiceConfig) { c.Workspace = "" }, func(c *ServiceConfig) { c.Workspace = "/" }, func(c *ServiceConfig) { c.HomeDir = "relative" }, func(c *ServiceConfig) { c.AppDataDir = "" }, func(c *ServiceConfig) {
		c.Scan = func(context.Context) (model.ScanResult, error) { return model.ScanResult{}, errors.New("failed") }
	}} {
		config := valid
		change(&config)
		if _, err := NewService(config); err == nil {
			t.Fatal("invalid config accepted")
		}
	}
}

// Real files and fix transactions expose incorrect paths, missing hash checks,
// incomplete rollback, or early publication; only the injected scanner varies.
func serviceFixture(t *testing.T, count int, scanHook func(int) error) (*Service, []string, *atomic.Int64) {
	t.Helper()
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	if err := os.Mkdir(workspace, 0o700); err != nil {
		t.Fatal(err)
	}
	paths := []string{}
	sources := []model.ConfigSource{}
	for i := 0; i < count; i++ {
		name := string(rune('a'+i)) + ".md"
		path := filepath.Join(workspace, name)
		if err := os.WriteFile(path, []byte("keep\nkeep\n"), 0o640); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
		sources = append(sources, model.ConfigSource{ID: name, LogicalPath: path, CanonicalPath: path, Client: "codex", Scope: model.ScopeProject, Format: model.FormatMarkdown, Exists: true, Readable: true, Kind: "instruction", DiscoveryReason: "fixture"})
	}
	calls := &atomic.Int64{}
	s, err := NewService(ServiceConfig{Workspace: workspace, HomeDir: root, AppDataDir: filepath.Join(root, "data"), Clock: func() time.Time { return time.Date(2026, 10, 9, 1, 2, 3, 0, time.UTC) }, Scan: func(context.Context) (model.ScanResult, error) {
		n := int(calls.Add(1))
		if scanHook != nil {
			if err := scanHook(n); err != nil {
				return model.ScanResult{}, err
			}
		}
		return model.ScanResult{SchemaVersion: model.ReportSchemaVersion, Analysis: model.Analysis{Sources: sources}}, nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	return s, paths, calls
}

func TestServiceRescanFailureAndConcurrentRevisions(t *testing.T) {
	fail := atomic.Bool{}
	s, _, _ := serviceFixture(t, 0, func(int) error {
		if fail.Load() {
			return errors.New("scan failed")
		}
		return nil
	})
	before := s.State()
	fail.Store(true)
	if _, err := s.Rescan(context.Background(), 1); err == nil {
		t.Fatal("failed scan accepted")
	}
	if !reflect.DeepEqual(before, s.State()) {
		t.Fatal("failed scan published state")
	}
	fail.Store(false)
	var workers sync.WaitGroup
	var successes atomic.Int64
	for i := 0; i < 8; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for j := 0; j < 20; j++ {
				if s.State().Revision < 1 {
					t.Error("partial state")
				}
			}
		}()
	}
	for i := 0; i < 8; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			_, err := s.Rescan(context.Background(), 1)
			if err == nil {
				successes.Add(1)
			} else if !errors.Is(err, ErrStaleRevision) {
				t.Error(err)
			}
		}()
	}
	workers.Wait()
	if successes.Load() != 1 || s.State().Revision != 2 {
		t.Fatalf("non-atomic revision: successes=%d state=%d", successes.Load(), s.State().Revision)
	}
}

func TestServiceStaleOperationsBeforeFilesystemAccess(t *testing.T) {
	s, paths, calls := serviceFixture(t, 1, nil)
	state := s.State()
	// An inaccessible backup root turns any accidental filesystem access into
	// an error other than ErrStaleRevision, without depending on permissions.
	if err := os.MkdirAll(s.config.AppDataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.config.AppDataDir, "backups"), []byte("block"), 0o600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	operations := map[string]func() error{
		"rescan":   func() error { _, e := s.Rescan(context.Background(), 0); return e },
		"plan":     func() error { _, e := s.PlanFixes(context.Background(), 0, nil); return e },
		"apply":    func() error { _, e := s.ApplyFixes(context.Background(), 0, []string{state.FixPlans[0].ID}); return e },
		"rollback": func() error { _, e := s.Rollback(context.Background(), 0, "bad"); return e },
		"save":     func() error { _, e := s.SaveSnapshot(context.Background(), 0, "baseline"); return e },
		"compare":  func() error { _, e := s.CompareSnapshot(context.Background(), 0, "baseline"); return e },
		"export":   func() error { _, e := s.Export(context.Background(), 0, &output, "0.2.0"); return e },
	}
	for name, op := range operations {
		t.Run(name, func(t *testing.T) {
			if err := op(); !errors.Is(err, ErrStaleRevision) {
				t.Fatalf("got %v", err)
			}
		})
	}
	if calls.Load() != 1 || output.Len() != 0 || !reflect.DeepEqual(state, s.State()) {
		t.Fatal("stale operation had effects")
	}
	data, _ := os.ReadFile(paths[0])
	if string(data) != "keep\nkeep\n" {
		t.Fatal("stale apply changed file")
	}
	if _, err := os.Stat(filepath.Join(s.config.AppDataDir, "snapshots")); !os.IsNotExist(err) {
		t.Fatal("stale save touched store")
	}
}

func TestServiceApplyPublishesOnce(t *testing.T) {
	var s *Service
	s, paths, _ := serviceFixture(t, 2, func(int) error {
		if s != nil && s.State().Revision != 1 {
			return errors.New("published during apply")
		}
		return nil
	})
	before := s.State()
	ids := []string{before.FixPlans[0].ID, before.FixPlans[1].ID}
	applied, err := s.ApplyFixes(context.Background(), 1, ids)
	if err != nil {
		t.Fatal(err)
	}
	if applied.Revision != 2 || len(applied.FixPlans) != 0 || len(applied.Backups) != 2 {
		t.Fatalf("apply: %+v", applied)
	}
	for _, path := range paths {
		data, _ := os.ReadFile(path)
		if string(data) != "keep\n" {
			t.Fatalf("not applied: %s", data)
		}
	}
	applied.Backups[0].ID = "mutated"
	if s.State().Backups[0].ID == "mutated" {
		t.Fatal("backup metadata aliases state")
	}
}

func TestServiceApplyFailureRestoresEveryEarlierTransaction(t *testing.T) {
	s, paths, _ := serviceFixture(t, 3, func(n int) error {
		if n == 5 {
			return errors.New("third verification failed")
		}
		return nil
	})
	before := s.State()
	ids := []string{}
	for _, plan := range before.FixPlans {
		ids = append(ids, plan.ID)
	}
	if _, err := s.ApplyFixes(context.Background(), 1, ids); err == nil {
		t.Fatal("verification failure accepted")
	}
	if !reflect.DeepEqual(before, s.State()) {
		t.Fatal("failed transaction request published state")
	}
	for _, path := range paths {
		data, _ := os.ReadFile(path)
		info, _ := os.Stat(path)
		if string(data) != "keep\nkeep\n" || info.Mode().Perm() != 0o640 {
			t.Fatalf("rollback missed %s", path)
		}
	}
}

func TestServiceRejectsUnnamedUnknownAndDuplicateFixIDs(t *testing.T) {
	s, paths, calls := serviceFixture(t, 1, nil)
	id := s.State().FixPlans[0].ID
	for _, ids := range [][]string{nil, {}, {"../bad"}, {"FIX-DUP-00000000"}, {id, id}} {
		if _, err := s.ApplyFixes(context.Background(), 1, ids); err == nil {
			t.Fatalf("accepted %v", ids)
		}
	}
	data, _ := os.ReadFile(paths[0])
	if string(data) != "keep\nkeep\n" || calls.Load() != 1 {
		t.Fatal("invalid IDs had effects")
	}
}

func TestServicePlanRefreshesAndSanitizesWithoutPublishing(t *testing.T) {
	s, paths, _ := serviceFixture(t, 1, nil)
	before := s.State()
	if err := os.WriteFile(paths[0], []byte("new\nnew\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	plans, err := s.PlanFixes(context.Background(), 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 1 || plans[0].Edits[0].ExpectedHash == before.FixPlans[0].Edits[0].ExpectedHash || plans[0].Edits[0].TargetPath != "./a.md" {
		t.Fatalf("plans: %+v", plans)
	}
	if !reflect.DeepEqual(before, s.State()) {
		t.Fatal("planning published state")
	}
}

func TestServiceSnapshotDriftAndExportUsePublicState(t *testing.T) {
	s, paths, calls := serviceFixture(t, 1, nil)
	meta, err := s.SaveSnapshot(context.Background(), 1, "baseline")
	if err != nil {
		t.Fatal(err)
	}
	if meta.Path != "" || meta.Name != "baseline" || s.State().Revision != 1 {
		t.Fatalf("metadata exposes internal path or changes revision: %+v", meta)
	}
	data, err := os.ReadFile(filepath.Join(s.config.AppDataDir, "snapshots", "baseline.json"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte(s.config.HomeDir)) || !bytes.Contains(data, []byte("./a.md")) {
		t.Fatalf("unsafe snapshot: %s", data)
	}
	if err := os.WriteFile(paths[0], []byte("changed externally\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	state, err := s.CompareSnapshot(context.Background(), 1, "baseline")
	if err != nil {
		t.Fatal(err)
	}
	if state.Revision != 2 || state.Drift == nil || state.Drift.Baseline != "baseline" || len(state.Drift.Changes) != 0 || calls.Load() != 1 {
		t.Fatalf("compare did not use public state: %+v", state)
	}
	state.Drift.Baseline = "mutated"
	if s.State().Drift.Baseline != "baseline" {
		t.Fatal("drift aliases state")
	}
	var output bytes.Buffer
	if _, err := s.Export(context.Background(), 2, &output, "0.2.0"); err != nil {
		t.Fatal(err)
	}
	archive, err := zip.NewReader(bytes.NewReader(output.Bytes()), int64(output.Len()))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, file := range archive.File {
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(data, []byte(s.config.HomeDir)) {
			t.Fatalf("export leaked path: %s", file.Name)
		}
		if file.Name == "report.json" {
			found = true
			if !bytes.Contains(data, []byte("remove duplicate line: keep")) {
				t.Fatal("export rescanned raw workspace")
			}
		}
	}
	if !found || calls.Load() != 1 || s.State().Revision != 2 {
		t.Fatal("export did not consume current public state")
	}
	if _, err := s.Rescan(context.Background(), 2); err != nil {
		t.Fatal(err)
	}
	if s.State().Drift != nil {
		t.Fatal("rescan retained obsolete drift")
	}
}

func TestServiceRollbackValidatesIDAndPublishesAfterRescan(t *testing.T) {
	s, paths, _ := serviceFixture(t, 1, nil)
	for _, id := range []string{"", "../backup", "20261301T000000Z-0000000000000000"} {
		if _, err := s.Rollback(context.Background(), 1, id); err == nil {
			t.Fatalf("accepted backup %q", id)
		}
	}
	state, err := s.ApplyFixes(context.Background(), 1, []string{s.State().FixPlans[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	restored, err := s.Rollback(context.Background(), 2, state.Backups[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Revision != 3 || len(restored.FixPlans) != 1 {
		t.Fatalf("rollback state: %+v", restored)
	}
	data, _ := os.ReadFile(paths[0])
	info, _ := os.Stat(paths[0])
	if string(data) != "keep\nkeep\n" || info.Mode().Perm() != 0o640 {
		t.Fatal("rollback failed to restore bytes and mode")
	}
}

func TestServiceInitialStateIsSanitizedAndImmutable(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	if err := os.Mkdir(workspace, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(workspace, "AGENTS.md")
	if err := os.WriteFile(path, []byte("hello\nhello\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	raw := model.ScanResult{Analysis: model.Analysis{
		Sources: []model.ConfigSource{{ID: "source", LogicalPath: path, CanonicalPath: path, Exists: true, Readable: true}},
		Graph:   model.Graph{Nodes: []model.ConfigNode{{ID: "node", Attributes: map[string]model.SafeValue{"path": {Display: filepath.Join(root, "private"), Present: true}}}}},
	}}
	calls := 0
	s, err := NewService(ServiceConfig{Workspace: workspace, HomeDir: root, AppDataDir: filepath.Join(root, "data"), Clock: func() time.Time { return time.Date(2026, 10, 9, 1, 2, 3, 0, time.UTC) }, Scan: func(context.Context) (model.ScanResult, error) { calls++; return raw, nil }})
	if err != nil {
		t.Fatal(err)
	}
	state := s.State()
	if calls != 1 || state.Revision != 1 || state.ScannedAt != "2026-10-09T01:02:03Z" || state.Workspace != "." {
		t.Fatalf("initial state: calls=%d state=%+v", calls, state)
	}
	encoded, _ := json.Marshal(state)
	if strings.Contains(string(encoded), root) || state.Result.Analysis.Sources[0].CanonicalPath != "./AGENTS.md" || state.FixPlans[0].Edits[0].TargetPath != "./AGENTS.md" {
		t.Fatalf("unsafe public state: %s", encoded)
	}
	state.Result.Analysis.Graph.Nodes[0].Attributes["path"] = model.SafeValue{Display: "mutated"}
	state.FixPlans[0].Edits[0].TargetPath = "mutated"
	raw.Analysis.Sources[0].ID = "mutated raw"
	fresh := s.State()
	if fresh.Result.Analysis.Graph.Nodes[0].Attributes["path"].Display != "~/private" || fresh.FixPlans[0].Edits[0].TargetPath != "./AGENTS.md" || fresh.Result.Analysis.Sources[0].ID != "source" {
		t.Fatalf("caller changed state: %+v", fresh)
	}
}
