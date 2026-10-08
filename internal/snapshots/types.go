// Package snapshots persists sanitized reports and compares their normalized state.
package snapshots

import "time"

const SchemaVersion = "1.0.0"

const (
	ChangeAdded         = "ADDED"
	ChangeRemoved       = "REMOVED"
	ChangeChanged       = "CHANGED"
	EntityClient        = "CLIENT"
	EntitySource        = "SOURCE"
	EntityNode          = "NODE"
	EntityFinding       = "FINDING"
	EntityCompatibility = "COMPATIBILITY"
)

type Change struct {
	Kind       string   `json:"kind"`
	EntityType string   `json:"entity_type"`
	ID         string   `json:"id"`
	Summary    string   `json:"summary"`
	Clients    []string `json:"clients,omitempty"`
}

type Diff struct {
	SchemaVersion string   `json:"schema_version"`
	Baseline      string   `json:"baseline,omitempty"`
	Changes       []Change `json:"changes"`
}

// Metadata describes a local baseline. Path is for internal file access only.
type Metadata struct {
	Name          string    `json:"name"`
	Path          string    `json:"-"`
	SchemaVersion string    `json:"schema_version"`
	CreatedAt     time.Time `json:"created_at"`
}
