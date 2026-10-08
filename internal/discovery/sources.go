package discovery

import (
	"os"
	"path/filepath"

	"github.com/Z-lab-boop/harnessscope/internal/model"
)

func (e Environment) InspectKnownPath(logical string) model.ConfigSource {
	logical = filepath.Clean(logical)
	source := model.ConfigSource{LogicalPath: logical}
	info, err := os.Lstat(logical)
	if err != nil {
		return source
	}
	source.Exists = true
	source.Symlink = info.Mode()&os.ModeSymlink != 0

	canonical, err := filepath.EvalSymlinks(logical)
	if err != nil {
		return source
	}
	canonical, err = filepath.Abs(canonical)
	if err != nil {
		return source
	}
	source.CanonicalPath = canonical
	targetInfo, err := os.Stat(canonical)
	if err != nil || targetInfo.IsDir() {
		return source
	}
	file, err := os.Open(canonical)
	if err != nil {
		return source
	}
	_ = file.Close()
	source.Readable = true
	return source
}
