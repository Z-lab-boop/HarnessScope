// Package export creates sanitized, offline diagnostic ZIP bundles.
package export

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Z-lab-boop/harnessscope/internal/model"
	"github.com/Z-lab-boop/harnessscope/internal/report"
	"github.com/Z-lab-boop/harnessscope/internal/secrets"
	"github.com/Z-lab-boop/harnessscope/internal/snapshots"
)

type Input struct {
	ToolVersion string
	Report      model.ScanResult
	Drift       *snapshots.Diff
}
type ClientTier struct {
	ID   string           `json:"id"`
	Tier model.ClientTier `json:"tier"`
}
type MemberHash struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
}
type Manifest struct {
	SchemaVersion string       `json:"schema_version"`
	ToolVersion   string       `json:"tool_version"`
	GeneratedAt   string       `json:"generated_at"`
	Clients       []ClientTier `json:"clients"`
	Members       []MemberHash `json:"members"`
}

const SchemaVersion = "1.0.0"

const readme = "HarnessScope offline diagnostic bundle\n\nThis bundle is sanitized and contains reports, optional normalized drift, and SHA-256 member hashes. It contains no raw configuration or backup contents. Human review is required before public upload. Review all members for sensitive project details before sharing.\n"

var homePathPattern = regexp.MustCompile(`(?i)(?:/(?:Users|home)/|[A-Z]:[\\/](?:Users|Documents and Settings)[\\/])`)

type Exporter struct{ clock func() time.Time }

func New(clock func() time.Time) *Exporter {
	if clock == nil {
		clock = time.Now
	}
	return &Exporter{clock: clock}
}

type member struct {
	name string
	data []byte
}

// Write accepts a caller-sanitized report, retains the authoritative report
// writers, and rejects any unsafe generated member before touching output.
// Payloads are lexicographically ordered; the manifest is always written last.
func (e *Exporter) Write(ctx context.Context, output io.Writer, input Input) (Manifest, error) {
	if err := ctx.Err(); err != nil {
		return Manifest{}, err
	}
	if strings.TrimSpace(input.ToolVersion) == "" {
		return Manifest{}, fmt.Errorf("tool version is empty")
	}
	generatedAt := e.clock().UTC()
	var reportJSON, reportHTML bytes.Buffer
	if err := report.WriteJSON(&reportJSON, input.Report); err != nil {
		return Manifest{}, err
	}
	if err := report.WriteHTML(&reportHTML, input.Report); err != nil {
		return Manifest{}, err
	}
	members := []member{{"README.txt", []byte(readme)}, {"report.html", reportHTML.Bytes()}, {"report.json", reportJSON.Bytes()}}
	if input.Drift != nil {
		data, err := json.MarshalIndent(input.Drift, "", "  ")
		if err != nil {
			return Manifest{}, err
		}
		members = append(members, member{"drift.json", append(data, '\n')})
	}
	sort.Slice(members, func(i, j int) bool { return members[i].name < members[j].name })
	manifest := Manifest{SchemaVersion: SchemaVersion, ToolVersion: input.ToolVersion, GeneratedAt: generatedAt.Format(time.RFC3339Nano), Clients: []ClientTier{}, Members: []MemberHash{}}
	for _, client := range input.Report.Analysis.Clients {
		if client.ID == "" || (client.Compatibility.Tier != model.TierVerified && client.Compatibility.Tier != model.TierPreview) {
			return Manifest{}, fmt.Errorf("invalid client compatibility tier")
		}
		manifest.Clients = append(manifest.Clients, ClientTier{ID: client.ID, Tier: client.Compatibility.Tier})
	}
	sort.Slice(manifest.Clients, func(i, j int) bool {
		if manifest.Clients[i].ID != manifest.Clients[j].ID {
			return manifest.Clients[i].ID < manifest.Clients[j].ID
		}
		return manifest.Clients[i].Tier < manifest.Clients[j].Tier
	})
	for _, item := range members {
		sum := sha256.Sum256(item.data)
		manifest.Members = append(manifest.Members, MemberHash{Name: item.name, SHA256: hex.EncodeToString(sum[:])})
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return Manifest{}, err
	}
	members = append(members, member{"manifest.json", append(data, '\n')})
	redactor := secrets.NewRedactor()
	home, _ := os.UserHomeDir()
	for _, item := range members {
		if err := ctx.Err(); err != nil {
			return Manifest{}, err
		}
		if !safeMember(item, redactor, home) {
			return Manifest{}, fmt.Errorf("bundle member %s contains sensitive data", item.name)
		}
	}
	archive := zip.NewWriter(output)
	for _, item := range members {
		if err := ctx.Err(); err != nil {
			return Manifest{}, err
		}
		header := zip.FileHeader{Name: item.name, Method: zip.Store, Modified: generatedAt}
		header.SetMode(0o600)
		writer, err := archive.CreateHeader(&header)
		if err != nil {
			return Manifest{}, err
		}
		if _, err := writer.Write(item.data); err != nil {
			return Manifest{}, err
		}
	}
	if err := archive.Close(); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func safeMember(item member, redactor secrets.Redactor, home string) bool {
	if !safeText(string(item.data), redactor, home) {
		return false
	}
	// JSON escapes can conceal credential URL separators or Windows paths from
	// a byte-only check, so also inspect decoded keys and string values.
	if strings.HasSuffix(item.name, ".json") {
		var value any
		if json.Unmarshal(item.data, &value) != nil {
			return false
		}
		return safeJSON(value, redactor, home)
	}
	return true
}

func safeJSON(value any, redactor secrets.Redactor, home string) bool {
	switch typed := value.(type) {
	case string:
		return safeText(typed, redactor, home)
	case []any:
		for _, child := range typed {
			if !safeJSON(child, redactor, home) {
				return false
			}
		}
	case map[string]any:
		for key, child := range typed {
			if !safeText(key, redactor, home) || !safeJSON(child, redactor, home) {
				return false
			}
		}
	}
	return true
}

func safeText(value string, redactor secrets.Redactor, home string) bool {
	if strings.Contains(value, "HARNESSSCOPE-CANARY") || strings.Contains(value, "HARNESSCOPE-CANARY") || homePathPattern.MatchString(value) {
		return false
	}
	if home != "" && home != "/" && (value == home || strings.Contains(value, home+"/")) {
		return false
	}
	return redactor.ScrubText(value) == value
}
