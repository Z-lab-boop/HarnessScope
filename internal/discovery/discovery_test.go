package discovery

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewEnvironmentUsesPlatformAppDataWithoutRealHome(t *testing.T) {
	darwin := NewEnvironment("darwin", "/synthetic/home", nil)
	linux := NewEnvironment("linux", "/synthetic/home", nil)

	if got, want := darwin.AppDataDir, "/synthetic/home/Library/Application Support/HarnessScope"; got != want {
		t.Fatalf("darwin app data=%q want=%q", got, want)
	}
	if got, want := linux.AppDataDir, "/synthetic/home/.local/share/harnessscope"; got != want {
		t.Fatalf("linux app data=%q want=%q", got, want)
	}
}

func TestWalkAncestorsReturnsWorkspaceToFilesystemRoot(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "repo", "nested")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := WalkAncestors(workspace)
	if err != nil {
		t.Fatal(err)
	}
	if got[0] != workspace {
		t.Fatalf("first ancestor=%q want=%q", got[0], workspace)
	}
	if got[len(got)-1] != string(filepath.Separator) {
		t.Fatalf("last ancestor=%q want filesystem root", got[len(got)-1])
	}
}

func TestInspectKnownPathRetainsLogicalAndCanonicalSymlinkPaths(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target.toml")
	logical := filepath.Join(root, "config.toml")
	if err := os.WriteFile(target, []byte("model = \"test\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, logical); err != nil {
		t.Fatal(err)
	}
	canonicalTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}

	got := NewEnvironment("darwin", root, nil).InspectKnownPath(logical)
	if got.LogicalPath != logical || got.CanonicalPath != canonicalTarget || !got.Symlink || !got.Readable {
		t.Fatalf("unexpected source: %#v", got)
	}
}

func TestInspectKnownPathRepresentsMissingAndCyclicSources(t *testing.T) {
	root := t.TempDir()
	env := NewEnvironment("linux", root, nil)
	missing := env.InspectKnownPath(filepath.Join(root, "missing.json"))
	if missing.Exists || missing.Readable {
		t.Fatalf("missing source marked present: %#v", missing)
	}

	a := filepath.Join(root, "a")
	b := filepath.Join(root, "b")
	if err := os.Symlink(b, a); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(a, b); err != nil {
		t.Fatal(err)
	}
	cyclic := env.InspectKnownPath(a)
	if !cyclic.Exists || cyclic.Readable || !cyclic.Symlink {
		t.Fatalf("cyclic source was not bounded: %#v", cyclic)
	}
}

func TestFindExecutableUsesOnlyInjectedPath(t *testing.T) {
	bin := t.TempDir()
	executable := filepath.Join(bin, "codex")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	env := NewEnvironment("darwin", "/synthetic/home", []string{bin})

	got := env.FindExecutable("codex")
	if !got.Installed || got.Executable != executable {
		t.Fatalf("unexpected detection: %#v", got)
	}
}
