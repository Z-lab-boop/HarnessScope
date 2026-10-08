package fixes

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Z-lab-boop/harnessscope/internal/model"
)

type VerifyFunc func(context.Context, []string) error

type TransactionResult struct {
	BackupID string
	Paths    []string
}

type manifest struct {
	BackupID string          `json:"backup_id"`
	Entries  []manifestEntry `json:"entries"`
}

type manifestEntry struct {
	Target     string      `json:"target"`
	BackupFile string      `json:"backup_file"`
	Mode       os.FileMode `json:"mode"`
}

func Apply(ctx context.Context, plan model.FixPlan, backupRoot string, verify VerifyFunc) (TransactionResult, error) {
	if plan.Risk != model.RiskSafe {
		return TransactionResult{}, fmt.Errorf("fix %s is not SAFE", plan.ID)
	}
	if len(plan.Edits) == 0 {
		return TransactionResult{}, fmt.Errorf("fix %s has no edits", plan.ID)
	}
	for _, edit := range plan.Edits {
		data, err := os.ReadFile(edit.TargetPath)
		if err != nil {
			return TransactionResult{}, fmt.Errorf("read fix target: %w", err)
		}
		if hashBytes(data) != edit.ExpectedHash {
			return TransactionResult{}, fmt.Errorf("target changed since the fix was planned")
		}
	}
	backupID, err := newBackupID()
	if err != nil {
		return TransactionResult{}, err
	}
	directory := filepath.Join(backupRoot, backupID)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return TransactionResult{}, err
	}
	manifestData := manifest{BackupID: backupID}
	seen := make(map[string]struct{})
	for _, edit := range plan.Edits {
		if _, ok := seen[edit.TargetPath]; ok {
			continue
		}
		seen[edit.TargetPath] = struct{}{}
		data, err := os.ReadFile(edit.TargetPath)
		if err != nil {
			return TransactionResult{}, err
		}
		info, err := os.Stat(edit.TargetPath)
		if err != nil {
			return TransactionResult{}, err
		}
		backupName := fmt.Sprintf("file-%03d.bak", len(manifestData.Entries)+1)
		if err := os.WriteFile(filepath.Join(directory, backupName), data, 0o600); err != nil {
			return TransactionResult{}, err
		}
		manifestData.Entries = append(manifestData.Entries, manifestEntry{Target: edit.TargetPath, BackupFile: backupName, Mode: info.Mode().Perm()})
	}
	encoded, err := json.MarshalIndent(manifestData, "", "  ")
	if err != nil {
		return TransactionResult{}, err
	}
	if err := os.WriteFile(filepath.Join(directory, "manifest.json"), append(encoded, '\n'), 0o600); err != nil {
		return TransactionResult{}, err
	}

	paths := make([]string, 0, len(plan.Edits))
	for _, edit := range plan.Edits {
		if err := applyEdit(edit); err != nil {
			_ = Rollback(ctx, backupRoot, backupID)
			return TransactionResult{}, err
		}
		paths = append(paths, edit.TargetPath)
	}
	if verify != nil {
		if err := verify(ctx, paths); err != nil {
			_ = Rollback(ctx, backupRoot, backupID)
			return TransactionResult{}, fmt.Errorf("postcondition verification failed: %w", err)
		}
	}
	return TransactionResult{BackupID: backupID, Paths: paths}, nil
}

func Rollback(_ context.Context, backupRoot, backupID string) error {
	directory := filepath.Join(backupRoot, filepath.Base(backupID))
	data, err := os.ReadFile(filepath.Join(directory, "manifest.json"))
	if err != nil {
		return err
	}
	var stored manifest
	if err := json.Unmarshal(data, &stored); err != nil {
		return err
	}
	if stored.BackupID != backupID {
		return fmt.Errorf("backup manifest ID mismatch")
	}
	for _, entry := range stored.Entries {
		backup, err := os.ReadFile(filepath.Join(directory, filepath.Base(entry.BackupFile)))
		if err != nil {
			return err
		}
		if err := atomicWrite(entry.Target, backup, entry.Mode); err != nil {
			return err
		}
	}
	return nil
}

func applyEdit(edit model.Edit) error {
	switch edit.Operation {
	case "chmod_executable":
		return os.Chmod(edit.TargetPath, os.FileMode(edit.ResultMode))
	case "remove_byte_range", "normalize_path":
		data, err := os.ReadFile(edit.TargetPath)
		if err != nil {
			return err
		}
		if edit.StartOffset < 0 || edit.EndOffset < edit.StartOffset || edit.EndOffset > len(data) {
			return fmt.Errorf("edit range is outside target")
		}
		replacement := []byte(nil)
		if edit.Operation == "normalize_path" {
			replacement = []byte(edit.Replacement)
		}
		updated := append(append(append([]byte(nil), data[:edit.StartOffset]...), replacement...), data[edit.EndOffset:]...)
		info, err := os.Stat(edit.TargetPath)
		if err != nil {
			return err
		}
		return atomicWrite(edit.TargetPath, updated, info.Mode().Perm())
	default:
		return fmt.Errorf("unsupported fix operation %q", edit.Operation)
	}
}

func atomicWrite(target string, data []byte, mode os.FileMode) error {
	directory := filepath.Dir(target)
	temporary, err := os.CreateTemp(directory, ".hscope-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(mode); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, target)
}

func newBackupID() (string, error) {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	return time.Now().UTC().Format("20060102T150405Z") + "-" + hex.EncodeToString(random[:]), nil
}
