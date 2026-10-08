package discovery

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Z-lab-boop/harnessscope/internal/model"
)

type Environment struct {
	GOOS            string
	HomeDir         string
	AppDataDir      string
	SystemConfigDir string
	PathEntries     []string
}

func NewEnvironment(goos, home string, pathEntries []string) Environment {
	home = filepath.Clean(home)
	appData := filepath.Join(home, ".local", "share", "harnessscope")
	if goos == "darwin" {
		appData = filepath.Join(home, "Library", "Application Support", "HarnessScope")
	}
	return Environment{
		GOOS:            goos,
		HomeDir:         home,
		AppDataDir:      appData,
		SystemConfigDir: "/etc/codex",
		PathEntries:     append([]string(nil), pathEntries...),
	}
}

func CurrentEnvironment() (Environment, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Environment{}, err
	}
	env := NewEnvironment(runtime.GOOS, home, filepath.SplitList(os.Getenv("PATH")))
	if runtime.GOOS == "linux" {
		if xdg := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); xdg != "" {
			env.AppDataDir = filepath.Join(xdg, "harnessscope")
		}
	}
	return env, nil
}

func (e Environment) FindExecutable(name string) model.DetectionResult {
	if name == "" || filepath.Base(name) != name {
		return model.DetectionResult{Error: "executable name must not contain a path"}
	}
	for _, directory := range e.PathEntries {
		candidate := filepath.Join(directory, name)
		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() || info.Mode().Perm()&0o111 == 0 {
			continue
		}
		absolute, err := filepath.Abs(candidate)
		if err != nil {
			return model.DetectionResult{Error: err.Error()}
		}
		return model.DetectionResult{Installed: true, Executable: absolute}
	}
	return model.DetectionResult{Installed: false}
}

func WalkAncestors(cwd string) ([]string, error) {
	if strings.TrimSpace(cwd) == "" {
		return nil, errors.New("working directory is empty")
	}
	current, err := filepath.Abs(cwd)
	if err != nil {
		return nil, err
	}
	current = filepath.Clean(current)
	var ancestors []string
	for {
		ancestors = append(ancestors, current)
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return ancestors, nil
}
