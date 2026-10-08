package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/Z-lab-boop/harnessscope/internal/export"
	"github.com/Z-lab-boop/harnessscope/internal/fixes"
	"github.com/Z-lab-boop/harnessscope/internal/model"
	"github.com/Z-lab-boop/harnessscope/internal/sanitize"
	"github.com/Z-lab-boop/harnessscope/internal/snapshots"
)

type Service struct {
	// Operations always acquire opMu before mu. External callbacks and I/O
	// never run with mu held, so a scan closure may safely call State.
	opMu   sync.Mutex
	mu     sync.RWMutex
	config ServiceConfig
	raw    model.ScanResult
	public DashboardState
}

func NewService(config ServiceConfig) (*Service, error) {
	if config.Scan == nil {
		return nil, fmt.Errorf("dashboard scan function is required")
	}
	for _, path := range []string{config.Workspace, config.HomeDir, config.AppDataDir} {
		if !filepath.IsAbs(path) || filepath.Clean(path) == string(filepath.Separator) {
			return nil, fmt.Errorf("dashboard roots must be absolute non-root paths")
		}
	}
	config.Workspace = filepath.Clean(config.Workspace)
	config.HomeDir = filepath.Clean(config.HomeDir)
	config.AppDataDir = filepath.Clean(config.AppDataDir)
	if config.Clock == nil {
		config.Clock = time.Now
	}
	s := &Service{config: config}
	raw, err := config.Scan(context.Background())
	if err != nil {
		return nil, err
	}
	owned, state, err := s.prepare(raw, 1)
	if err != nil {
		return nil, err
	}
	s.publish(owned, state)
	return s, nil
}

// State returns a deep copy; no maps, slices, or pointers alias service state.
func (s *Service) State() DashboardState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return copyState(s.public)
}

// Rescan publishes exactly once, and leaves the old state intact on failure.
func (s *Service) Rescan(ctx context.Context, expectedRevision uint64) (DashboardState, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	if err := s.checkRevision(ctx, expectedRevision); err != nil {
		return DashboardState{}, err
	}
	raw, err := s.config.Scan(ctx)
	if err != nil {
		return DashboardState{}, err
	}
	owned, state, err := s.prepare(raw, expectedRevision+1)
	if err != nil {
		return DashboardState{}, err
	}
	if err := ctx.Err(); err != nil {
		return DashboardState{}, err
	}
	s.publish(owned, state)
	return s.State(), nil
}

// PlanFixes refreshes plans without publishing a new scan revision. An empty
// selection previews all plans; ApplyFixes always requires explicit IDs.
func (s *Service) PlanFixes(ctx context.Context, expectedRevision uint64, ids []string) ([]model.FixPlan, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	if err := s.checkRevision(ctx, expectedRevision); err != nil {
		return nil, err
	}
	if err := validateFixIDs(ids); err != nil {
		return nil, err
	}
	raw, err := s.config.Scan(ctx)
	if err != nil {
		return nil, err
	}
	plans, err := fixes.Plan(raw, ids)
	if err != nil {
		return nil, err
	}
	raw.Analysis.FixPlans = plans
	public, err := sanitize.Report(raw, s.config.HomeDir, s.config.Workspace)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if public.Analysis.FixPlans == nil {
		return []model.FixPlan{}, nil
	}
	return public.Analysis.FixPlans, nil
}

// ApplyFixes verifies every selected transaction before a single publication.
// Failure unwinds successful transactions in reverse order; fixes.Apply owns
// rollback of the currently failing transaction.
func (s *Service) ApplyFixes(ctx context.Context, expectedRevision uint64, ids []string) (DashboardState, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	if err := s.checkRevision(ctx, expectedRevision); err != nil {
		return DashboardState{}, err
	}
	if len(ids) == 0 {
		return DashboardState{}, fmt.Errorf("select at least one SAFE fix ID")
	}
	if err := validateFixIDs(ids); err != nil {
		return DashboardState{}, err
	}
	published := s.State()
	for _, id := range ids {
		found := false
		for _, plan := range published.FixPlans {
			if plan.ID == id && plan.Risk == model.RiskSafe {
				found = true
				break
			}
		}
		if !found {
			return DashboardState{}, fmt.Errorf("selected SAFE fix ID was not found")
		}
	}
	raw, err := s.config.Scan(ctx)
	if err != nil {
		return DashboardState{}, err
	}
	plans, err := fixes.Plan(raw, ids)
	if err != nil {
		return DashboardState{}, err
	}
	for _, plan := range plans {
		if plan.Risk != model.RiskSafe {
			return DashboardState{}, fmt.Errorf("selected fix is not SAFE")
		}
	}
	backupRoot := filepath.Join(s.config.AppDataDir, "backups")
	applied := []string{}
	unwind := func(cause error) (DashboardState, error) {
		// Cancellation must not prevent restoring files already changed.
		for i := len(applied) - 1; i >= 0; i-- {
			if err := fixes.Rollback(context.WithoutCancel(ctx), backupRoot, applied[i]); err != nil {
				cause = errors.Join(cause, fmt.Errorf("rollback %s failed: %w", applied[i], err))
			}
		}
		return DashboardState{}, cause
	}
	for _, plan := range plans {
		if err := ctx.Err(); err != nil {
			return unwind(err)
		}
		transaction, err := fixes.Apply(ctx, plan, backupRoot, func(ctx context.Context, _ []string) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			current, err := s.config.Scan(ctx)
			if err != nil {
				return err
			}
			remaining, err := fixes.Plan(current, nil)
			if err != nil {
				return err
			}
			for _, pending := range remaining {
				if pending.ID == plan.ID {
					return fmt.Errorf("fix postcondition is not satisfied")
				}
			}
			raw = current
			return ctx.Err()
		})
		if err != nil {
			return unwind(err)
		}
		applied = append(applied, transaction.BackupID)
	}
	owned, state, err := s.prepare(raw, expectedRevision+1)
	if err != nil {
		return unwind(err)
	}
	if err := ctx.Err(); err != nil {
		return unwind(err)
	}
	s.publish(owned, state)
	return s.State(), nil
}

// Rollback only accepts a validated ID listed by the authoritative history
// reader. A failed follow-up scan does not publish a misleading new revision.
func (s *Service) Rollback(ctx context.Context, expectedRevision uint64, backupID string) (DashboardState, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	if err := s.checkRevision(ctx, expectedRevision); err != nil {
		return DashboardState{}, err
	}
	if !backupIDPattern.MatchString(backupID) {
		return DashboardState{}, fmt.Errorf("invalid backup ID")
	}
	backupRoot := filepath.Join(s.config.AppDataDir, "backups")
	backups, err := fixes.ListBackups(backupRoot)
	if err != nil {
		return DashboardState{}, err
	}
	found := false
	for _, backup := range backups {
		if backup.ID == backupID {
			found = true
			break
		}
	}
	if !found {
		return DashboardState{}, fmt.Errorf("backup ID was not found")
	}
	if err := fixes.Rollback(ctx, backupRoot, backupID); err != nil {
		return DashboardState{}, err
	}
	raw, err := s.config.Scan(ctx)
	if err != nil {
		return DashboardState{}, err
	}
	owned, state, err := s.prepare(raw, expectedRevision+1)
	if err != nil {
		return DashboardState{}, err
	}
	if err := ctx.Err(); err != nil {
		return DashboardState{}, err
	}
	s.publish(owned, state)
	return s.State(), nil
}

func (s *Service) SaveSnapshot(ctx context.Context, expectedRevision uint64, name string) (snapshots.Metadata, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	if err := s.checkRevision(ctx, expectedRevision); err != nil {
		return snapshots.Metadata{}, err
	}
	meta, err := s.snapshotStore().Save(name, s.State().Result)
	meta.Path = "" // Metadata.Path is a store-internal filesystem capability.
	return meta, err
}

func (s *Service) CompareSnapshot(ctx context.Context, expectedRevision uint64, name string) (DashboardState, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	if err := s.checkRevision(ctx, expectedRevision); err != nil {
		return DashboardState{}, err
	}
	baseline, err := s.snapshotStore().Load(name)
	if err != nil {
		return DashboardState{}, err
	}
	state := s.State()
	diff := snapshots.Compare(state.Result, baseline)
	diff.Baseline = name
	state.Drift, state.Revision = &diff, expectedRevision+1
	if err := ctx.Err(); err != nil {
		return DashboardState{}, err
	}
	s.mu.RLock()
	raw := s.raw // Immutable, private, and never modified by comparison.
	s.mu.RUnlock()
	s.publish(raw, state)
	return s.State(), nil
}

func (s *Service) Export(ctx context.Context, expectedRevision uint64, output io.Writer, toolVersion string) (export.Manifest, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	if err := s.checkRevision(ctx, expectedRevision); err != nil {
		return export.Manifest{}, err
	}
	if output == nil {
		return export.Manifest{}, fmt.Errorf("export writer is required")
	}
	state := s.State()
	return export.New(s.config.Clock).Write(ctx, output, export.Input{ToolVersion: toolVersion, Report: state.Result, Drift: state.Drift})
}

func (s *Service) snapshotStore() *snapshots.Store {
	return snapshots.NewStore(filepath.Join(s.config.AppDataDir, "snapshots"), s.config.Clock)
}

// The caller must hold opMu through validation, all I/O and publication.
func (s *Service) checkRevision(ctx context.Context, expected uint64) error {
	s.mu.RLock()
	revision := s.public.Revision
	s.mu.RUnlock()
	if expected != revision {
		return ErrStaleRevision
	}
	if revision == ^uint64(0) {
		return fmt.Errorf("dashboard revision exhausted")
	}
	return ctx.Err()
}

var fixIDPattern = regexp.MustCompile(`^FIX-[A-Z]+-[A-F0-9]{8}$`)
var backupIDPattern = regexp.MustCompile(`^[0-9]{8}T[0-9]{6}Z-[0-9a-f]{16}$`)

func validateFixIDs(ids []string) error {
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if !fixIDPattern.MatchString(id) || seen[id] {
			return fmt.Errorf("invalid or duplicate fix ID")
		}
		seen[id] = true
	}
	return nil
}

func (s *Service) prepare(raw model.ScanResult, revision uint64) (model.ScanResult, DashboardState, error) {
	data, err := model.MarshalCanonical(raw)
	if err != nil {
		return model.ScanResult{}, DashboardState{}, err
	}
	var owned model.ScanResult
	if err := json.Unmarshal(data, &owned); err != nil {
		return model.ScanResult{}, DashboardState{}, err
	}
	plans, err := fixes.Plan(owned, nil)
	if err != nil {
		return model.ScanResult{}, DashboardState{}, err
	}
	// Include plans in the shared report sanitizer, keeping transaction paths
	// private while giving reports, snapshots and the dashboard identical plans.
	withPlans := owned
	withPlans.Analysis.FixPlans = plans
	public, err := sanitize.Report(withPlans, s.config.HomeDir, s.config.Workspace)
	if err != nil {
		return model.ScanResult{}, DashboardState{}, err
	}
	backups, err := fixes.ListBackups(filepath.Join(s.config.AppDataDir, "backups"))
	if err != nil {
		return model.ScanResult{}, DashboardState{}, err
	}
	safePlans := public.Analysis.FixPlans
	if safePlans == nil {
		safePlans = []model.FixPlan{}
	}
	state := DashboardState{SchemaVersion: DashboardSchemaVersion, Revision: revision,
		ScannedAt: s.config.Clock().UTC().Format(time.RFC3339Nano), Workspace: ".",
		Result: public, FixPlans: safePlans, Backups: backups}
	return owned, copyState(state), nil
}

func (s *Service) publish(raw model.ScanResult, state DashboardState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.raw, s.public = raw, copyState(state)
}

func copyState(state DashboardState) DashboardState {
	// DashboardState consists solely of JSON-compatible model primitives.
	// Marshal/Unmarshal cannot fail for this closed value graph.
	state.Result = model.Canonicalize(state.Result)
	data, err := json.Marshal(state)
	if err != nil {
		panic(err)
	}
	var copy DashboardState
	if err := json.Unmarshal(data, &copy); err != nil {
		panic(err)
	}
	return copy
}
