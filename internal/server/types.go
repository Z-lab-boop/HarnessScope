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
