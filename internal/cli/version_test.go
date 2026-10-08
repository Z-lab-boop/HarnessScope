package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/Z-lab-boop/harnessscope/internal/buildinfo"
)

func TestRuntimeAndVersionCommandUseBuildVersion(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	runtime, err := DefaultRuntime()
	if err != nil {
		t.Fatal(err)
	}
	if runtime.ToolVersion != buildinfo.Version {
		t.Fatalf("runtime version %q != build version %q", runtime.ToolVersion, buildinfo.Version)
	}
	var stdout, stderr bytes.Buffer
	if code := ExecuteWithRuntime(context.Background(), []string{"--version"}, &stdout, &stderr, runtime); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), buildinfo.Version) {
		t.Fatalf("version output %q", stdout.String())
	}
}
