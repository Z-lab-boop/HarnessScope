package fixes

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Z-lab-boop/harnessscope/internal/model"
)

func TestDuplicateFixApplyRescanRollbackRoundTrip(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "AGENTS.md")
	original := "keep this\nduplicate rule\nduplicate rule\n"
	if err := os.WriteFile(target, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	result := scanResultForSource(target)
	plans, err := Plan(result, nil)
	if err != nil {
		t.Fatal(err)
	}
	plan := findOperation(t, plans, "remove_byte_range")
	backupRoot := filepath.Join(root, "backups")

	transaction, err := Apply(context.Background(), plan, backupRoot, func(context.Context, []string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	changed, _ := os.ReadFile(target)
	if string(changed) != "keep this\nduplicate rule\n" {
		t.Fatalf("unexpected fixed content: %q", changed)
	}
	manifest, err := os.ReadFile(filepath.Join(backupRoot, transaction.BackupID, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(manifest), "HARNESSSCOPE-CANARY") {
		t.Fatalf("secret leaked to manifest: %s", manifest)
	}
	for _, path := range []string{
		filepath.Join(backupRoot, transaction.BackupID, "manifest.json"),
		filepath.Join(backupRoot, transaction.BackupID, "file-001.bak"),
	} {
		info, statErr := os.Stat(path)
		if statErr != nil {
			t.Fatalf("stat backup %s: %v", path, statErr)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("backup permission mismatch for %s: mode=%v", path, info.Mode().Perm())
		}
	}
	if err := Rollback(context.Background(), backupRoot, transaction.BackupID); err != nil {
		t.Fatal(err)
	}
	restored, _ := os.ReadFile(target)
	if string(restored) != original {
		t.Fatalf("rollback mismatch: %q", restored)
	}
}

func TestDuplicatePlanRemovesEveryRepeatedLineInOneTransaction(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "AGENTS.md")
	if err := os.WriteFile(target, []byte("keep\nsame\nsame\nsame\ntail\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	plans, err := Plan(scanResultForSource(target), nil)
	if err != nil {
		t.Fatal(err)
	}
	plan := findOperation(t, plans, "remove_byte_range")
	if len(plan.Edits) != 2 {
		t.Fatalf("expected two duplicate removals in one transaction, got %#v", plan.Edits)
	}
	if _, err := Apply(context.Background(), plan, filepath.Join(root, "backups"), nil); err != nil {
		t.Fatal(err)
	}
	changed, _ := os.ReadFile(target)
	if string(changed) != "keep\nsame\ntail\n" {
		t.Fatalf("unexpected fixed content: %q", changed)
	}
}

func TestApplyRefusesConcurrentModification(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "AGENTS.md")
	if err := os.WriteFile(target, []byte("same\nsame\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	plans, err := Plan(scanResultForSource(target), nil)
	if err != nil {
		t.Fatal(err)
	}
	plan := findOperation(t, plans, "remove_byte_range")
	if err := os.WriteFile(target, []byte("changed\nsame\nsame\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err = Apply(context.Background(), plan, filepath.Join(root, "backups"), nil)
	if !errors.Is(err, ErrTargetChanged) {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "backups")); !os.IsNotExist(err) {
		t.Fatal("conflicting target created a backup")
	}
}

func TestApplyAutomaticallyRollsBackFailedVerification(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "AGENTS.md")
	original := "same\nsame\n"
	if err := os.WriteFile(target, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	plans, _ := Plan(scanResultForSource(target), nil)
	plan := findOperation(t, plans, "remove_byte_range")

	_, err := Apply(context.Background(), plan, filepath.Join(root, "backups"), func(context.Context, []string) error {
		return errors.New("postcondition failed")
	})
	if err == nil {
		t.Fatal("expected verification failure")
	}
	restored, _ := os.ReadFile(target)
	if string(restored) != original {
		t.Fatalf("automatic rollback failed: %q", restored)
	}
}

func TestPlanRepairsExistingShebangHookMode(t *testing.T) {
	root := t.TempDir()
	hook := filepath.Join(root, "check.sh")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\nexit 0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	result := model.ScanResult{Analysis: model.Analysis{Graph: model.Graph{Nodes: []model.ConfigNode{{
		ID: "hook", Type: model.NodeHook, Client: "claude", DisplayName: "hook.PreToolUse",
		Attributes: map[string]model.SafeValue{"command": {Display: hook, Present: true}},
	}}}}}

	plans, err := Plan(result, nil)
	if err != nil {
		t.Fatal(err)
	}
	plan := findOperation(t, plans, "chmod_executable")
	if _, err := Apply(context.Background(), plan, filepath.Join(root, "backups"), nil); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(hook)
	if info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("hook is still not executable: %o", info.Mode().Perm())
	}
}

func scanResultForSource(path string) model.ScanResult {
	return model.ScanResult{Analysis: model.Analysis{Sources: []model.ConfigSource{{
		ID: "source_fixture", LogicalPath: path, CanonicalPath: path, Exists: true, Readable: true,
		Client: "codex", Scope: model.ScopeProject, Format: model.FormatMarkdown, Kind: "instruction",
	}}}}
}

func findOperation(t *testing.T, plans []model.FixPlan, operation string) model.FixPlan {
	t.Helper()
	for _, plan := range plans {
		for _, edit := range plan.Edits {
			if edit.Operation == operation {
				return plan
			}
		}
	}
	t.Fatalf("operation %q not found in %#v", operation, plans)
	return model.FixPlan{}
}
