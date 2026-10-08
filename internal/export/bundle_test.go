package export

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Z-lab-boop/harnessscope/internal/model"
	"github.com/Z-lab-boop/harnessscope/internal/report"
	"github.com/Z-lab-boop/harnessscope/internal/snapshots"
)

func TestBundleDeterministicMembersHashesModesAndOfflineReport(t *testing.T) {
	instant := time.Date(2026, 10, 9, 12, 34, 56, 0, time.FixedZone("fixture", 8*3600))
	for _, withDrift := range []bool{false, true} {
		t.Run(map[bool]string{false: "without_drift", true: "with_drift"}[withDrift], func(t *testing.T) {
			input := bundleInput()
			if withDrift {
				input.Drift = &snapshots.Diff{SchemaVersion: "1.0.0", Baseline: "baseline", Changes: []snapshots.Change{}}
			}
			exporter := New(func() time.Time { return instant })
			var first, second bytes.Buffer
			manifest, err := exporter.Write(context.Background(), &first, input)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := exporter.Write(context.Background(), &second, input); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(first.Bytes(), second.Bytes()) {
				t.Fatal("identical inputs produce different ZIP bytes")
			}
			files := readBundle(t, first.Bytes())
			wantNames := []string{"README.txt", "report.html", "report.json", "manifest.json"}
			if withDrift {
				wantNames = []string{"README.txt", "drift.json", "report.html", "report.json", "manifest.json"}
			}
			reader, _ := zip.NewReader(bytes.NewReader(first.Bytes()), int64(first.Len()))
			var names []string
			for _, file := range reader.File {
				names = append(names, file.Name)
				if file.Method != zip.Store || file.Mode().Perm() != 0o600 {
					t.Fatalf("unsafe/nondeterministic header: %#v", file.FileHeader)
				}
				if !file.Modified.Equal(instant) {
					t.Fatalf("timestamp = %v, want %v", file.Modified, instant)
				}
			}
			if !reflect.DeepEqual(names, wantNames) {
				t.Fatalf("member order = %v, want %v", names, wantNames)
			}
			var stored Manifest
			if err := json.Unmarshal(files["manifest.json"], &stored); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(stored, manifest) {
				t.Fatal("returned manifest differs from archive")
			}
			if manifest.SchemaVersion != "1.0.0" || manifest.ToolVersion != "0.2.0" || manifest.GeneratedAt != "2026-10-09T04:34:56Z" {
				t.Fatalf("manifest = %#v", manifest)
			}
			if !reflect.DeepEqual(manifest.Clients, []ClientTier{{ID: "claude", Tier: model.TierVerified}, {ID: "opencode", Tier: model.TierPreview}}) {
				t.Fatalf("client tiers = %#v", manifest.Clients)
			}
			if len(manifest.Members) != len(wantNames)-1 {
				t.Fatal("manifest does not cover every payload")
			}
			for i, member := range manifest.Members {
				if member.Name != wantNames[i] {
					t.Fatalf("hash order = %v", manifest.Members)
				}
				sum := sha256.Sum256(files[member.Name])
				if member.SHA256 != hex.EncodeToString(sum[:]) {
					t.Fatalf("hash mismatch: %s", member.Name)
				}
			}
			var authoritativeJSON, authoritativeHTML bytes.Buffer
			if err := report.WriteJSON(&authoritativeJSON, input.Report); err != nil {
				t.Fatal(err)
			}
			if err := report.WriteHTML(&authoritativeHTML, input.Report); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(files["report.json"], authoritativeJSON.Bytes()) || !bytes.Equal(files["report.html"], authoritativeHTML.Bytes()) {
				t.Fatal("export changed authoritative report output")
			}
			html := string(files["report.html"])
			if !strings.Contains(html, "OFFLINE REPORT") || !strings.Contains(html, "data-encoding=\"base64\"") || strings.Contains(html, "src=\"http") || strings.Contains(html, "href=\"http") {
				t.Fatal("HTML is not self-contained offline report")
			}
			readme := strings.ToLower(string(files["README.txt"]))
			if !strings.Contains(readme, "sanitized") || !strings.Contains(readme, "human review") || !strings.Contains(readme, "public upload") {
				t.Fatal("README missing sharing guidance")
			}
		})
	}
}

func TestBundleRejectsSensitiveGeneratedMembersBeforeWriting(t *testing.T) {
	for _, value := range []string{"HARNESSSCOPE-CANARY-secret", "/Users/private/.config", "/home/private/.config", `C:\Users\private\config`, "sk-abcdefghijklmnopqrstuvwxyz012345", "ghp_abcdefghijklmnopqrstuvwxyz012345", "Bearer abcdefghijklmnopqrstuvwxyz", "https://user:password@example.test"} {
		for _, location := range []string{"drift", "manifest"} {
			t.Run(location+"/"+value, func(t *testing.T) {
				input := bundleInput()
				if location == "drift" {
					input.Drift = &snapshots.Diff{SchemaVersion: "1.0.0", Changes: []snapshots.Change{{Summary: value}}}
				} else {
					input.ToolVersion = value
				}
				var output bytes.Buffer
				if _, err := New(nil).Write(context.Background(), &output, input); err == nil {
					t.Fatal("sensitive member accepted")
				} else if strings.Contains(err.Error(), value) {
					t.Fatal("error leaks sensitive value")
				}
				if output.Len() != 0 {
					t.Fatal("rejected export wrote partial archive")
				}
			})
		}
	}
	for _, value := range []string{"HARNESSSCOPE-CANARY-secret", "/Users/private/config"} {
		input := bundleInput()
		input.Report.Analysis.Findings = []model.Finding{{Summary: value}}
		var output bytes.Buffer
		if _, err := New(nil).Write(context.Background(), &output, input); err == nil || output.Len() != 0 {
			t.Fatalf("unsafe report accepted: %v", err)
		}
	}
}

func TestBundleKeepsReportCredentialRedaction(t *testing.T) {
	input := bundleInput()
	input.Report.Analysis.Findings = []model.Finding{{Summary: "sk-abcdefghijklmnopqrstuvwxyz012345"}}
	var output bytes.Buffer
	if _, err := New(nil).Write(context.Background(), &output, input); err != nil {
		t.Fatal(err)
	}
	files := readBundle(t, output.Bytes())
	if bytes.Contains(files["report.json"], []byte("sk-abcdefghijklmnopqrstuvwxyz012345")) || !bytes.Contains(files["report.json"], []byte("[REDACTED]")) {
		t.Fatal("authoritative report redaction lost")
	}
}

func TestBundleCancellationAndWriterFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var output bytes.Buffer
	if _, err := New(nil).Write(ctx, &output, bundleInput()); !errors.Is(err, context.Canceled) || output.Len() != 0 {
		t.Fatalf("cancellation = %v, bytes = %d", err, output.Len())
	}
	failure := errors.New("writer failed")
	if _, err := New(nil).Write(context.Background(), failingWriter{failure}, bundleInput()); !errors.Is(err, failure) {
		t.Fatalf("write error = %v", err)
	}
}

func TestBundleRejectsPrivateKeyAndNonstandardHome(t *testing.T) {
	t.Setenv("HOME", "/private/fixture-home")
	for _, value := range []string{
		"-----BEGIN PRIVATE KEY-----\nfixture\n-----END PRIVATE KEY-----",
		"/private/fixture-home/project",
		"/private/fixture-home",
		"HARNESSCOPE-CANARY-secret",
	} {
		input := bundleInput()
		input.Drift = &snapshots.Diff{SchemaVersion: "1.0.0", Baseline: value, Changes: []snapshots.Change{}}
		var output bytes.Buffer
		if _, err := New(nil).Write(context.Background(), &output, input); err == nil || output.Len() != 0 {
			t.Fatalf("unsafe escaped member accepted: %v", err)
		}
	}
}

func TestBundleRejectsInvalidManifestMetadata(t *testing.T) {
	for _, invalid := range []string{"empty_version", "empty_client", "unknown_tier"} {
		input := bundleInput()
		switch invalid {
		case "empty_version":
			input.ToolVersion = ""
		case "empty_client":
			input.Report.Analysis.Clients[0].ID = ""
		case "unknown_tier":
			input.Report.Analysis.Clients[0].Compatibility.Tier = "UNKNOWN"
		}
		var output bytes.Buffer
		if _, err := New(nil).Write(context.Background(), &output, input); err == nil || output.Len() != 0 {
			t.Fatalf("invalid metadata accepted: %s", invalid)
		}
	}
}

func TestBundleManifestHasOnlyPublicFields(t *testing.T) {
	input := bundleInput()
	input.Report.Analysis.Clients[0].Detection.Executable = "<PROJECT>/bin/client"
	var output bytes.Buffer
	if _, err := New(nil).Write(context.Background(), &output, input); err != nil {
		t.Fatal(err)
	}
	files := readBundle(t, output.Bytes())
	var object map[string]any
	if err := json.Unmarshal(files["manifest.json"], &object); err != nil {
		t.Fatal(err)
	}
	if len(object) != 5 {
		t.Fatalf("unexpected manifest fields: %v", object)
	}
	for _, entry := range object["clients"].([]any) {
		if len(entry.(map[string]any)) != 2 {
			t.Fatal("client entry contains nonpublic metadata")
		}
	}
	for _, entry := range object["members"].([]any) {
		if len(entry.(map[string]any)) != 2 {
			t.Fatal("hash entry contains nonpublic metadata")
		}
	}
	if bytes.Contains(files["manifest.json"], []byte("executable")) || bytes.Contains(files["manifest.json"], []byte("<PROJECT>")) {
		t.Fatal("manifest contains local metadata")
	}
}

type failingWriter struct{ err error }

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }

func bundleInput() Input {
	return Input{ToolVersion: "0.2.0", Report: model.ScanResult{SchemaVersion: "1.0.0", Analysis: model.Analysis{Clients: []model.ClientResult{{ID: "opencode", Compatibility: model.CompatibilityMetadata{Tier: model.TierPreview}}, {ID: "claude", Compatibility: model.CompatibilityMetadata{Tier: model.TierVerified}}}}}}
}

func readBundle(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for _, file := range reader.File {
		member, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(member)
		member.Close()
		if err != nil {
			t.Fatal(err)
		}
		files[file.Name] = content
	}
	return files
}
