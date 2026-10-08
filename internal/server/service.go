package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
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
		return DashboardState{}, fmt.Errorf("%w: select at least one SAFE fix ID", ErrInvalidRequest)
	}
	if err := validateFixIDs(ids); err != nil {
		return DashboardState{}, err
	}
	published := s.State()
	for _, id := range ids {
		found := false
		for _, plan := range published.FixPlans {
			if plan.ID == id {
				if plan.Risk != model.RiskSafe {
					return DashboardState{}, ErrUnsafeFix
				}
				found = true
				break
			}
		}
		if !found {
			return DashboardState{}, ErrNotFound
		}
	}
	raw, err := s.config.Scan(ctx)
	if err != nil {
		return DashboardState{}, err
	}
	plans, err := fixes.Plan(raw, nil)
	if err != nil {
		return DashboardState{}, err
	}
	selected := make([]model.FixPlan, 0, len(ids))
	for _, id := range ids {
		found := false
		for _, plan := range plans {
			if plan.ID != id {
				continue
			}
			if plan.Risk != model.RiskSafe {
				return DashboardState{}, ErrUnsafeFix
			}
			selected = append(selected, plan)
			found = true
			break
		}
		if !found {
			return DashboardState{}, ErrNotFound
		}
	}
	// Keep the planner's canonical transaction order, independent of UI order.
	sort.Slice(selected, func(i, j int) bool { return selected[i].ID < selected[j].ID })
	plans = selected
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
		return DashboardState{}, ErrInvalidRequest
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
		return DashboardState{}, ErrNotFound
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
	if !snapshotIDPattern.MatchString(name) {
		return snapshots.Metadata{}, ErrInvalidRequest
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
	if !snapshotIDPattern.MatchString(name) {
		return DashboardState{}, ErrInvalidRequest
	}
	baseline, err := s.snapshotStore().Load(name)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return DashboardState{}, ErrNotFound
		}
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
	return s.exportActive(ctx, expectedRevision, output, toolVersion, "")
}

func (s *Service) exportActive(ctx context.Context, expectedRevision uint64, output io.Writer, toolVersion, baseline string) (export.Manifest, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	if err := s.checkRevision(ctx, expectedRevision); err != nil {
		return export.Manifest{}, err
	}
	if output == nil {
		return export.Manifest{}, fmt.Errorf("export writer is required")
	}
	state := s.State()
	if baseline != "" {
		if !snapshotIDPattern.MatchString(baseline) {
			return export.Manifest{}, ErrInvalidRequest
		}
		if state.Drift == nil || state.Drift.Baseline != baseline {
			return export.Manifest{}, ErrStaleRevision
		}
	}
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
var snapshotIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
var publicIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

func validateFixIDs(ids []string) error {
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if !fixIDPattern.MatchString(id) || seen[id] {
			return fmt.Errorf("%w: invalid or duplicate fix ID", ErrInvalidRequest)
		}
		seen[id] = true
	}
	return nil
}

// Explain resolves only an exact public node ID and returns owned provenance.
func (s *Service) Explain(id string) (ExplainResponse, error) {
	if !publicIDPattern.MatchString(id) {
		return ExplainResponse{}, ErrInvalidRequest
	}
	state := s.State()
	for _, node := range state.Result.Analysis.Graph.Nodes {
		if node.ID != id {
			continue
		}
		result := ExplainResponse{Node: node, Edges: []model.Edge{}}
		for _, edge := range state.Result.Analysis.Graph.Edges {
			if edge.From == id || edge.To == id {
				result.Edges = append(result.Edges, edge)
			}
		}
		return result, nil
	}
	return ExplainResponse{}, ErrNotFound
}

// CompareClients uses only clients registered in one immutable public revision.
func (s *Service) CompareClients(left, right string) (ComparisonResponse, error) {
	if !publicIDPattern.MatchString(left) || !publicIDPattern.MatchString(right) {
		return ComparisonResponse{}, ErrInvalidRequest
	}
	state := s.State()
	clients := map[string]bool{}
	for _, client := range state.Result.Analysis.Clients {
		clients[client.ID] = true
	}
	if !clients[left] || !clients[right] {
		return ComparisonResponse{}, ErrNotFound
	}
	rows := map[string]*ComparisonRow{}
	for _, node := range state.Result.Analysis.Graph.Nodes {
		if (node.Client != left && node.Client != right) || node.Type == model.NodeClient || node.Type == model.NodeSource || node.DisplayName == "" {
			continue
		}
		key := string(node.Type) + "\x00" + node.DisplayName
		if rows[key] == nil {
			rows[key] = &ComparisonRow{Name: node.DisplayName, Type: node.Type}
		}
		row := rows[key]
		if node.Client == left && row.Left == nil {
			owned := node
			row.Left = &owned
		}
		if node.Client == right && row.Right == nil {
			owned := node
			row.Right = &owned
		}
	}
	result := ComparisonResponse{Left: left, Right: right, Rows: []ComparisonRow{}}
	for _, row := range rows {
		switch {
		case row.Left == nil || row.Right == nil:
			row.Status = "missing"
		case reflect.DeepEqual(row.Left.Attributes, row.Right.Attributes):
			row.Status = "same"
		default:
			row.Status = "different"
		}
		result.Rows = append(result.Rows, *row)
	}
	sort.Slice(result.Rows, func(i, j int) bool {
		if result.Rows[i].Name != result.Rows[j].Name {
			return result.Rows[i].Name < result.Rows[j].Name
		}
		return result.Rows[i].Type < result.Rows[j].Type
	})
	return result, nil
}

// ListSnapshots never returns store-internal filesystem capabilities.
func (s *Service) ListSnapshots(ctx context.Context) ([]snapshots.Metadata, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	items, err := s.snapshotStore().List()
	for i := range items {
		items[i].Path = ""
	}
	return items, err
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
