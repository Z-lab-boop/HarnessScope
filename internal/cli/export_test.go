package cli

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBundleWriteFailureCleansPartialFileAndPreservesForcedDestination(t *testing.T) {
	for _, force := range []bool{false, true} {
		t.Run(map[bool]string{false: "exclusive", true: "force"}[force], func(t *testing.T) {
			dir := t.TempDir()
			target := filepath.Join(dir, "bundle.zip")
			if force {
				if err := os.WriteFile(target, []byte("original"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			failure := errors.New("writer failed after partial output")
			err := writeBundleAtomically(target, force, func(w io.Writer) error {
				if _, err := w.Write([]byte("partial zip")); err != nil {
					t.Fatal(err)
				}
				if force {
					data, _ := os.ReadFile(target)
					if string(data) != "original" {
						t.Fatal("force truncated live destination")
					}
				}
				return failure
			})
			if !errors.Is(err, failure) {
				t.Fatalf("error=%v", err)
			}
			if force {
				data, _ := os.ReadFile(target)
				if string(data) != "original" {
					t.Fatal("destination changed after failure")
				}
			} else if _, err := os.Stat(target); !os.IsNotExist(err) {
				t.Fatal("partial file retained")
			}
			items := directoryNames(t, dir)
			if len(items) != map[bool]int{false: 0, true: 1}[force] {
				t.Fatalf("temporary files: %v", items)
			}
		})
	}
}

func TestBundleForceRenameFailureCleansTemporary(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "existing-directory")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeBundleAtomically(target, true, func(w io.Writer) error { _, err := w.Write([]byte("new")); return err }); err == nil {
		t.Fatal("replaced directory")
	}
	if got := directoryNames(t, dir); len(got) != 1 || got[0] != "existing-directory" {
		t.Fatalf("rename failure left temporary: %v", got)
	}
}

func TestExportCommandsMembersOverwriteAndNoPollution(t *testing.T) {
	workspace := t.TempDir()
	output := filepath.Join(t.TempDir(), "diagnostic.zip")
	runtime := fixtureRuntime(t, commandFixtureAdapter{id: "codex"})
	runtime.ToolVersion = "0.2.0"
	runDiagnostic(t, runtime, ExitOK, "snapshot", "save", "base", workspace)
	for _, baseline := range []string{"", "base"} {
		args := []string{"export", workspace, "--output", output}
		if baseline != "" {
			args = append(args, "--baseline", baseline, "--force")
		}
		if got := strings.TrimSpace(runDiagnostic(t, runtime, ExitOK, args...)); got != output {
			t.Fatalf("stdout=%q", got)
		}
		archive, err := zip.OpenReader(output)
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, f := range archive.File {
			names = append(names, f.Name)
			r, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			data, err := io.ReadAll(r)
			r.Close()
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(data, []byte(workspace)) || bytes.Contains(data, []byte(runtime.Environment.HomeDir)) {
				t.Fatalf("leaked roots in %s", f.Name)
			}
		}
		archive.Close()
		want := "README.txt,report.html,report.json,manifest.json"
		if baseline != "" {
			want = "README.txt,drift.json,report.html,report.json,manifest.json"
		}
		if strings.Join(names, ",") != want {
			t.Fatalf("members=%v", names)
		}
		info, _ := os.Stat(output)
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("mode=%v", info.Mode())
		}
	}
	before, _ := os.ReadFile(output)
	runDiagnostic(t, runtime, ExitOperational, "export", workspace, "--output", output)
	after, _ := os.ReadFile(output)
	if !bytes.Equal(before, after) {
		t.Fatal("refused overwrite changed output")
	}
	runtime.ToolVersion = "bad/version"
	runDiagnostic(t, runtime, ExitOperational, "export", workspace, "--output", output, "--force")
	after, _ = os.ReadFile(output)
	if !bytes.Equal(before, after) {
		t.Fatal("failed force export changed output")
	}
	failed := filepath.Join(filepath.Dir(output), "failed.zip")
	runDiagnostic(t, runtime, ExitOperational, "export", workspace, "--output", failed)
	if _, err := os.Stat(failed); !os.IsNotExist(err) {
		t.Fatalf("failed output retained: %v", err)
	}
	if got := directoryNames(t, filepath.Dir(output)); len(got) != 1 || got[0] != "diagnostic.zip" {
		t.Fatalf("export temporaries retained: %v", got)
	}
	if got := directoryNames(t, workspace); len(got) != 0 {
		t.Fatalf("workspace pollution: %v", got)
	}
	runDiagnostic(t, runtime, ExitOperational, "export", workspace)
}
