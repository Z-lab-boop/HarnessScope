package fixes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"time"
)

// BackupSummary intentionally excludes target paths and backup contents.
type BackupSummary struct {
	ID        string `json:"id"`
	CreatedAt string `json:"created_at"`
	FileCount int    `json:"file_count"`
}

var backupIDPattern = regexp.MustCompile(`^[0-9]{8}T[0-9]{6}Z-[0-9a-f]{16}$`)
var backupFilePattern = regexp.MustCompile(`^file-[0-9]{3,}\.bak$`)

// ListBackups reads only v0.1 transaction manifests below validated backup IDs.
// Invalid unrelated entries are ignored; rollback retains its strict behavior
// when a particular backup is selected.
func ListBackups(root string) ([]BackupSummary, error) {
	items := []BackupSummary{}
	if root == "" {
		return nil, fmt.Errorf("backup root is empty")
	}
	info, err := os.Lstat(root)
	if os.IsNotExist(err) {
		return items, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("backup root is not a directory")
	}
	directory, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer directory.Close()
	openedInfo, err := directory.Stat(".")
	if err != nil {
		return nil, err
	}
	if !os.SameFile(info, openedInfo) {
		return nil, fmt.Errorf("backup root changed while opening")
	}
	listing, err := directory.Open(".")
	if err != nil {
		return nil, err
	}
	defer listing.Close()
	entries, err := listing.ReadDir(-1)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		id := entry.Name()
		if !entry.IsDir() || !backupIDPattern.MatchString(id) {
			continue
		}
		createdAt, err := time.Parse("20060102T150405Z", id[:16])
		if err != nil {
			continue
		}
		stored, err := readHistoryManifest(directory, id)
		if err != nil {
			continue
		}
		items = append(items, BackupSummary{ID: id, CreatedAt: createdAt.Format(time.RFC3339), FileCount: len(stored.Entries)})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID > items[j].ID })
	return items, nil
}

func readHistoryManifest(root *os.Root, id string) (manifest, error) {
	info, err := root.Lstat(id)
	if err != nil || !info.IsDir() {
		return manifest{}, fmt.Errorf("invalid backup directory")
	}
	directory, err := root.OpenRoot(id)
	if err != nil {
		return manifest{}, err
	}
	defer directory.Close()
	openedInfo, err := directory.Stat(".")
	if err != nil || !os.SameFile(info, openedInfo) {
		return manifest{}, fmt.Errorf("backup directory changed while opening")
	}
	info, err = directory.Lstat("manifest.json")
	if err != nil || !info.Mode().IsRegular() {
		return manifest{}, fmt.Errorf("invalid backup manifest file")
	}
	file, err := directory.Open("manifest.json")
	if err != nil {
		return manifest{}, err
	}
	defer file.Close()
	openedInfo, err = file.Stat()
	if err != nil || !openedInfo.Mode().IsRegular() || !os.SameFile(info, openedInfo) {
		return manifest{}, fmt.Errorf("backup manifest changed while opening")
	}
	data, err := io.ReadAll(io.LimitReader(file, 4<<20+1))
	if err != nil {
		return manifest{}, err
	}
	if len(data) > 4<<20 {
		return manifest{}, fmt.Errorf("backup manifest exceeds size limit")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var stored manifest
	if err := decoder.Decode(&stored); err != nil {
		return manifest{}, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return manifest{}, fmt.Errorf("backup manifest has trailing data")
	}
	if stored.BackupID != id || stored.Entries == nil {
		return manifest{}, fmt.Errorf("invalid backup manifest")
	}
	// A zero permission mask is valid, but a missing/null mode is not a v0.1
	// manifest entry. Use pointers to distinguish absence from that valid zero.
	var required struct {
		Entries []struct {
			Mode *os.FileMode `json:"mode"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(data, &required); err != nil {
		return manifest{}, err
	}
	for _, entry := range required.Entries {
		if entry.Mode == nil {
			return manifest{}, fmt.Errorf("backup manifest entry lacks mode")
		}
	}
	for _, entry := range stored.Entries {
		if entry.Target == "" || !backupFilePattern.MatchString(entry.BackupFile) || entry.Mode > 0o777 {
			return manifest{}, fmt.Errorf("invalid backup manifest entry")
		}
	}
	return stored, nil
}
