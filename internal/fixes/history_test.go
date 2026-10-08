package fixes

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestListBackupsOnlyReturnsValidRedactedSummaries(t *testing.T) {
	root := t.TempDir()
	oldID, newID := "20261008T010203Z-0123456789abcdef", "20261009T040506Z-fedcba9876543210"
	entry := `{"target":"/Users/private/secret.txt","backup_file":"file-001.bak","mode":384}`
	writeHistoryManifest(t, root, oldID, `{"backup_id":"`+oldID+`","entries":[`+entry+`]}`)
	writeHistoryManifest(t, root, newID, `{"backup_id":"`+newID+`","entries":[`+entry+`,`+entry+`]}`)
	writeHistoryManifest(t, root, "unrelated", `{broken`)
	writeHistoryManifest(t, root, "..-traversal", `{"backup_id":"..-traversal","entries":[]}`)
	writeHistoryManifest(t, root, "20260230T010203Z-0123456789abcdef", `{"backup_id":"20260230T010203Z-0123456789abcdef","entries":[]}`)
	badID := "20261009T050000Z-0123456789abcdef"
	writeHistoryManifest(t, root, badID, `{broken`)
	mismatchID := "20261009T060000Z-0123456789abcdef"
	writeHistoryManifest(t, root, mismatchID, `{"backup_id":"`+oldID+`","entries":[]}`)
	traversalID := "20261009T070000Z-0123456789abcdef"
	writeHistoryManifest(t, root, traversalID, `{"backup_id":"`+traversalID+`","entries":[{"target":"private","backup_file":"../outside","mode":384}]}`)
	missingID := "20261009T080000Z-0123456789abcdef"
	writeHistoryManifest(t, root, missingID, `{"backup_id":"`+missingID+`"}`)
	for _, id := range []string{oldID, newID} {
		if err := os.WriteFile(filepath.Join(root, id, "file-001.bak"), []byte("HARNESSSCOPE-CANARY-private-backup"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	got, err := ListBackups(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []BackupSummary{{ID: newID, CreatedAt: "2026-10-09T04:05:06Z", FileCount: 2}, {ID: oldID, CreatedAt: "2026-10-08T01:02:03Z", FileCount: 1}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("summaries = %#v, want %#v", got, want)
	}
	data, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"target", "backup_file", "/Users/", "HARNESSSCOPE-CANARY", "secret.txt"} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("history leaks %q", forbidden)
		}
	}
	if err := Rollback(context.Background(), root, badID); err == nil {
		t.Fatal("selected malformed backup must still fail rollback")
	}
}

func TestListBackupsSkipsSymlinkDirectoriesAndManifests(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	id := "20261009T010203Z-0123456789abcdef"
	writeHistoryManifest(t, outside, id, `{"backup_id":"`+id+`","entries":[]}`)
	if err := os.Symlink(filepath.Join(outside, id), filepath.Join(root, id)); err != nil {
		t.Fatal(err)
	}
	id2 := "20261009T020203Z-0123456789abcdef"
	if err := os.Mkdir(filepath.Join(root, id2), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, id, "manifest.json"), filepath.Join(root, id2, "manifest.json")); err != nil {
		t.Fatal(err)
	}
	got, err := ListBackups(root)
	if err != nil || len(got) != 0 {
		t.Fatalf("symlinks returned summaries: %#v, %v", got, err)
	}
}

func TestListBackupsMissingRootAndRootErrors(t *testing.T) {
	root := t.TempDir()
	got, err := ListBackups(filepath.Join(root, "missing"))
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("missing root = %#v, %v", got, err)
	}
	if _, err := ListBackups(""); err == nil {
		t.Fatal("empty root accepted")
	}
	file := filepath.Join(root, "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ListBackups(file); err == nil {
		t.Fatal("file root accepted")
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	if _, err := ListBackups(link); err == nil {
		t.Fatal("symlink root accepted")
	}
}

func TestListBackupsReadsExistingApplyManifest(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "AGENTS.md")
	if err := os.WriteFile(target, []byte("same\nsame\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	plans, err := Plan(scanResultForSource(target), nil)
	if err != nil {
		t.Fatal(err)
	}
	backupRoot := filepath.Join(root, "backups")
	transaction, err := Apply(context.Background(), findOperation(t, plans, "remove_byte_range"), backupRoot, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ListBackups(backupRoot)
	if err != nil || len(got) != 1 || got[0].ID != transaction.BackupID || got[0].FileCount != 1 {
		t.Fatalf("v0.1 history = %#v, %v", got, err)
	}
}

func TestListBackupsRejectsIncompleteAndExtendedManifests(t *testing.T) {
	root := t.TempDir()
	id := "20261009T010203Z-0123456789abcdef"
	for _, payload := range []string{
		`{"backup_id":"` + id + `","entries":[{"target":"private","backup_file":"file-001.bak"}]}`,
		`{"backup_id":"` + id + `","entries":[],"unknown":true}`,
		`{"backup_id":"` + id + `","entries":[]} {"extra":true}`,
		`{"backup_id":"` + id + `","entries":null}`,
		`{"backup_id":"` + id + `","entries":[{"target":"private","backup_file":"file-001.bak","mode":4095}]}`,
	} {
		writeHistoryManifest(t, root, id, payload)
		got, err := ListBackups(root)
		if err != nil || len(got) != 0 {
			t.Fatalf("invalid manifest returned summary: %#v, %v", got, err)
		}
	}
}

func writeHistoryManifest(t *testing.T, root, id, data string) {
	t.Helper()
	dir := filepath.Join(root, id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}
