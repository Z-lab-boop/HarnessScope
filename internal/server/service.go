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
	"strings"
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

// PlanFixes revalidates the immutable plans already authorized by this scan
// revision. Changed edits require Rescan; an empty selection previews all
// plans, while ApplyFixes always requires explicit IDs.
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
	s.mu.RLock()
	reviewed := append([]model.FixPlan(nil), s.raw.Analysis.FixPlans...)
	s.mu.RUnlock()
	if !sameReviewedPlans(reviewed, plans, ids) {
		// A revision authorizes one immutable set of exact edits. A second tab
		// may inspect it, but filesystem changes require Rescan so an older tab
		// cannot inherit replacement hashes through a same-ID preview.
		return nil, fixes.ErrTargetChanged
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
	redactPublicPlans(public.Analysis.FixPlans)
	return public.Analysis.FixPlans, nil
}

func sameReviewedPlans(reviewed, refreshed []model.FixPlan, ids []string) bool {
	expected := reviewed
	if len(ids) > 0 {
		wanted := make(map[string]bool, len(ids))
		for _, id := range ids {
			wanted[id] = true
		}
		expected = make([]model.FixPlan, 0, len(ids))
		for _, plan := range reviewed {
			if wanted[plan.ID] {
				expected = append(expected, plan)
			}
		}
	}
	return reflect.DeepEqual(expected, refreshed)
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
	publishedIDs := make(map[string]bool, len(published.FixPlans))
	for _, id := range ids {
		for _, plan := range published.FixPlans {
			if plan.ID == id {
				publishedIDs[id] = true
				if plan.Risk != model.RiskSafe {
					return DashboardState{}, ErrUnsafeFix
				}
				break
			}
		}
	}
	s.mu.RLock()
	reviewed := append([]model.FixPlan(nil), s.raw.Analysis.FixPlans...)
	s.mu.RUnlock()
	selected := make([]model.FixPlan, 0, len(ids))
	for _, id := range ids {
		found := false
		for _, plan := range reviewed {
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
			if publishedIDs[id] {
				// The ID was public at this revision but is no longer bound to an
				// exact private preview. Fail closed and require a new preview.
				return DashboardState{}, fixes.ErrTargetChanged
			}
			return DashboardState{}, ErrNotFound
		}
	}
	raw, err := s.config.Scan(ctx)
	if err != nil {
		return DashboardState{}, err
	}
	// Keep the planner's canonical transaction order, independent of UI order.
	sort.Slice(selected, func(i, j int) bool { return selected[i].ID < selected[j].ID })
	steps, err := prepareFixSteps(selected)
	if err != nil {
		return DashboardState{}, err
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
	finalFiles := map[string]fixFileState{}
	for _, step := range steps {
		plan := step.plan
		if err := ctx.Err(); err != nil {
			return unwind(err)
		}
		if err := verifyFixFiles(step.before); err != nil {
			return unwind(err)
		}
		transaction, err := fixes.Apply(ctx, plan, backupRoot, func(ctx context.Context, _ []string) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := verifyFixFiles(step.after); err != nil {
				return err
			}
			current, err := s.config.Scan(ctx)
			if err != nil {
				return err
			}
			// A scan callback or concurrent writer must not replace our pinned
			// output before it becomes the input to another selected transaction.
			if err := verifyFixFiles(step.after); err != nil {
				return err
			}
			raw = current
			return ctx.Err()
		})
		if err != nil {
			return unwind(err)
		}
		applied = append(applied, transaction.BackupID)
		for path, file := range step.after {
			finalFiles[path] = file
		}
	}
	owned, state, err := s.prepare(raw, expectedRevision+1)
	if err != nil {
		return unwind(err)
	}
	if err := ctx.Err(); err != nil {
		return unwind(err)
	}
	if err := verifyFixFiles(finalFiles); err != nil {
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
	return s.exportActive(ctx, expectedRevision, output, toolVersion, "", nil)
}

func (s *Service) exportActive(ctx context.Context, expectedRevision uint64, output io.Writer, toolVersion, baseline string, forbiddenStrings []string) (export.Manifest, error) {
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
	return export.New(s.config.Clock).Write(ctx, output, export.Input{ToolVersion: toolVersion, Report: state.Result, Drift: state.Drift, ForbiddenStrings: forbiddenStrings})
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
			rows[key] = &ComparisonRow{Name: comparisonSafeText(node.DisplayName), Type: node.Type}
		}
		row := rows[key]
		node = comparisonPublicNode(node)
		if node.Client == left {
			row.Left = append(row.Left, node)
		}
		if node.Client == right {
			row.Right = append(row.Right, node)
		}
	}
	result := ComparisonResponse{Left: left, Right: right, Rows: []ComparisonRow{}}
	for _, row := range rows {
		sort.Slice(row.Left, func(i, j int) bool { return comparisonNodeKey(row.Left[i]) < comparisonNodeKey(row.Left[j]) })
		sort.Slice(row.Right, func(i, j int) bool { return comparisonNodeKey(row.Right[i]) < comparisonNodeKey(row.Right[j]) })
		switch {
		case len(row.Left) == 0 || len(row.Right) == 0:
			row.Status = "Missing"
		case reflect.DeepEqual(comparisonSignatures(row.Left), comparisonSignatures(row.Right)):
			row.Status = "Present"
		default:
			row.Status = "Divergent"
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

func comparisonNodeKey(node model.ConfigNode) string {
	return comparisonAttributeSignature(node) + "\x00" + node.ID
}

func comparisonSignatures(nodes []model.ConfigNode) []string {
	values := make([]string, 0, len(nodes))
	for _, node := range nodes {
		values = append(values, comparisonAttributeSignature(node))
	}
	sort.Strings(values)
	return values
}

func comparisonAttributeSignature(node model.ConfigNode) string {
	type field struct {
		Key, Kind, SecretCategory, Display string
		Present                            bool
	}
	keys := make([]string, 0, len(node.Attributes))
	for key := range node.Attributes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	fields := make([]field, 0, len(keys))
	for _, key := range keys {
		value := node.Attributes[key]
		display := comparisonSafeValue(value)
		fields = append(fields, field{key, value.Kind, value.SecretCategory, display, value.Present})
	}
	data, _ := json.Marshal(fields)
	return string(data)
}

var comparisonHomePath = regexp.MustCompile(`/(Users|home)/[^\s<>"']+`)

func comparisonSafeText(value string) string {
	return comparisonHomePath.ReplaceAllString(value, "[REDACTED PATH]")
}

func comparisonSafeValue(value model.SafeValue) string {
	kind := strings.ToLower(value.Kind)
	if value.SecretCategory != "" || strings.Contains(kind, "secret") || strings.Contains(kind, "credential") || strings.Contains(kind, "redact") {
		return "[REDACTED]"
	}
	return comparisonSafeText(value.Display)
}

func comparisonPublicNode(node model.ConfigNode) model.ConfigNode {
	node.DisplayName = comparisonSafeText(node.DisplayName)
	node.LoadCondition = comparisonSafeText(node.LoadCondition)
	if node.Attributes != nil {
		attributes := make(map[string]model.SafeValue, len(node.Attributes))
		for key, value := range node.Attributes {
			value.Display = comparisonSafeValue(value)
			attributes[key] = value
		}
		node.Attributes = attributes
	}
	node.Origins = append([]model.Origin(nil), node.Origins...)
	for i := range node.Origins {
		node.Origins[i].LogicalPath = comparisonSafeText(node.Origins[i].LogicalPath)
		node.Origins[i].FieldPath = comparisonSafeText(node.Origins[i].FieldPath)
		node.Origins[i].Rule = comparisonSafeText(node.Origins[i].Rule)
	}
	return node
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

// Browser/report plans describe operations without publishing instruction bytes
// or replacement payloads. Raw plans stay private and remain bound to the
// revision/preview that authorized a later apply.
func redactPublicPlans(plans []model.FixPlan) {
	for i := range plans {
		for j := range plans[i].Edits {
			edit := &plans[i].Edits[j]
			edit.Replacement = ""
			if edit.Operation == "remove_byte_range" {
				edit.RedactedPatch = "remove duplicate line: [REDACTED]"
			}
		}
	}
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
	owned.Analysis.FixPlans = plans
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
	redactPublicPlans(safePlans)
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
