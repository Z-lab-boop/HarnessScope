package snapshots

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Z-lab-boop/harnessscope/internal/model"
	"github.com/Z-lab-boop/harnessscope/internal/report"
	"github.com/Z-lab-boop/harnessscope/internal/secrets"
)

var snapshotName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

type Store struct {
	root  string
	clock func() time.Time
}

func NewStore(root string, clock func() time.Time) *Store {
	if clock == nil {
		clock = time.Now
	}
	return &Store{root: root, clock: clock}
}

// Save accepts a report already sanitized by its caller. It scrubs recognizable
// credentials again, validates report compatibility, and atomically replaces a baseline.
func (s *Store) Save(name string, result model.ScanResult) (Metadata, error) {
	if err := validateName(name); err != nil {
		return Metadata{}, err
	}
	var output bytes.Buffer
	if err := report.WriteJSON(&output, result); err != nil {
		return Metadata{}, err
	}
	data := []byte(secrets.NewRedactor().ScrubText(output.String()))
	safe, err := report.ReadJSON(bytes.NewReader(data))
	if err != nil {
		return Metadata{}, err
	}
	root, err := s.rootPath()
	if err != nil {
		return Metadata{}, err
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return Metadata{}, err
	}
	directory, err := openDirectory(root)
	if err != nil {
		return Metadata{}, err
	}
	defer directory.Close()
	if err := directory.Chmod(0o700); err != nil {
		return Metadata{}, err
	}
	path := filepath.Join(root, name+".json")
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() {
			return Metadata{}, fmt.Errorf("snapshot %q is not a regular file", name)
		}
	} else if !os.IsNotExist(err) {
		return Metadata{}, err
	}
	temporary, err := os.CreateTemp(root, ".snapshot-*")
	if err != nil {
		return Metadata{}, err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	defer temporary.Close()
	if err := temporary.Chmod(0o600); err != nil {
		return Metadata{}, err
	}
	if _, err := temporary.Write(data); err != nil {
		return Metadata{}, err
	}
	createdAt := s.clock().UTC()
	if err := os.Chtimes(temporaryPath, createdAt, createdAt); err != nil {
		return Metadata{}, err
	}
	if err := temporary.Sync(); err != nil {
		return Metadata{}, err
	}
	if err := temporary.Close(); err != nil {
		return Metadata{}, err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return Metadata{}, err
	}
	if err := directory.Sync(); err != nil {
		return Metadata{}, err
	}
	return Metadata{Name: name, Path: path, SchemaVersion: safe.SchemaVersion, CreatedAt: createdAt}, nil
}

func (s *Store) List() ([]Metadata, error) {
	items := []Metadata{}
	root, err := s.rootPath()
	if err != nil {
		return nil, err
	}
	directory, err := openDirectory(root)
	if os.IsNotExist(err) {
		return items, nil
	}
	if err != nil {
		return nil, err
	}
	defer directory.Close()
	entries, err := directory.ReadDir(-1)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".json")
		if !snapshotName.MatchString(name) {
			continue
		}
		result, info, err := s.load(name)
		if err != nil {
			return nil, err
		}
		items = append(items, Metadata{Name: name, Path: filepath.Join(root, entry.Name()), SchemaVersion: result.SchemaVersion, CreatedAt: info.ModTime().UTC()})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items, nil
}

func (s *Store) Load(name string) (model.ScanResult, error) {
	result, _, err := s.load(name)
	return result, err
}

func (s *Store) load(name string) (model.ScanResult, os.FileInfo, error) {
	if err := validateName(name); err != nil {
		return model.ScanResult{}, nil, err
	}
	path, err := s.rootPath()
	if err != nil {
		return model.ScanResult{}, nil, err
	}
	directory, err := openDirectory(path)
	if err != nil {
		return model.ScanResult{}, nil, err
	}
	defer directory.Close()
	root, err := os.OpenRoot(path)
	if err != nil {
		return model.ScanResult{}, nil, err
	}
	defer root.Close()
	filename := name + ".json"
	info, err := root.Lstat(filename)
	if err != nil {
		return model.ScanResult{}, nil, err
	}
	if !info.Mode().IsRegular() {
		return model.ScanResult{}, nil, fmt.Errorf("snapshot %q is not a regular file", name)
	}
	file, err := root.Open(filename)
	if err != nil {
		return model.ScanResult{}, nil, err
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return model.ScanResult{}, nil, err
	}
	if !openedInfo.Mode().IsRegular() || !os.SameFile(info, openedInfo) {
		return model.ScanResult{}, nil, fmt.Errorf("snapshot %q changed while opening", name)
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return model.ScanResult{}, nil, err
	}
	result, err := report.ReadJSON(strings.NewReader(secrets.NewRedactor().ScrubText(string(data))))
	return result, openedInfo, err
}

func (s *Store) rootPath() (string, error) {
	if s.root == "" {
		return "", fmt.Errorf("snapshot root is empty")
	}
	return filepath.Abs(s.root)
}

func openDirectory(path string) (*os.File, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("snapshot root is not a directory")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	opened, err := file.Stat()
	if err != nil || !opened.IsDir() || !os.SameFile(info, opened) {
		file.Close()
		return nil, fmt.Errorf("snapshot root changed while opening")
	}
	return file, nil
}

func validateName(name string) error {
	if !snapshotName.MatchString(name) {
		return fmt.Errorf("invalid snapshot name: use 1–64 ASCII letters, digits, dots, underscores or hyphens, starting with a letter or digit")
	}
	return nil
}
