// Package server owns the dashboard's revisioned, sanitized public state.
package server

import (
	"context"
	"errors"
	"time"

	"github.com/Z-lab-boop/harnessscope/internal/fixes"
	"github.com/Z-lab-boop/harnessscope/internal/model"
	"github.com/Z-lab-boop/harnessscope/internal/snapshots"
)

const DashboardSchemaVersion = "1.0.0"

var ErrStaleRevision = errors.New("stale dashboard revision")

var (
	ErrInvalidRequest = errors.New("invalid request")
	ErrNotFound       = errors.New("item not found")
	ErrUnsafeFix      = errors.New("selected fix is not SAFE")
)

// APIError is the stable JSON envelope for all API failures, including security.
type APIError struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details"`
}

type RevisionRequest struct {
	Revision uint64 `json:"revision"`
}
type ApplyRequest struct {
	Revision uint64   `json:"revision"`
	FixIDs   []string `json:"fix_ids"`
}
type RollbackRequest struct {
	Revision uint64 `json:"revision"`
	BackupID string `json:"backup_id"`
}
type SnapshotRequest struct {
	Revision uint64 `json:"revision"`
	Name     string `json:"name"`
}
type DriftRequest struct {
	Revision uint64 `json:"revision"`
	Baseline string `json:"baseline"`
}
type ExportRequest struct {
	Revision uint64 `json:"revision"`
	Baseline string `json:"baseline,omitempty"`
}
type ExplainResponse struct {
	Node  model.ConfigNode `json:"node"`
	Edges []model.Edge     `json:"edges"`
}
type ComparisonResponse struct {
	Left  string          `json:"left"`
	Right string          `json:"right"`
	Rows  []ComparisonRow `json:"rows"`
}
type ComparisonRow struct {
	Name   string            `json:"name"`
	Type   model.NodeType    `json:"type"`
	Status string            `json:"status"`
	Left   *model.ConfigNode `json:"left,omitempty"`
	Right  *model.ConfigNode `json:"right,omitempty"`
}

type ScanFunc func(context.Context) (model.ScanResult, error)

type DashboardState struct {
	SchemaVersion string                `json:"schema_version"`
	Revision      uint64                `json:"revision"`
	ScannedAt     string                `json:"scanned_at"`
	Workspace     string                `json:"workspace"`
	Result        model.ScanResult      `json:"result"`
	FixPlans      []model.FixPlan       `json:"fix_plans"`
	Backups       []fixes.BackupSummary `json:"backups"`
	Drift         *snapshots.Diff       `json:"drift,omitempty"`
}

type ServiceConfig struct {
	Workspace  string
	HomeDir    string
	AppDataDir string
	Scan       ScanFunc
	Clock      func() time.Time
}
