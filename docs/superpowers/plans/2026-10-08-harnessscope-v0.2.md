# HarnessScope v0.2 Local Control Center Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a secure, technology-styled local Web control center, deeper evidence-aware diagnostics, configuration drift, browser-mediated SAFE fixes, and sanitized diagnostic exports while preserving the v0.1 CLI and single-binary offline release.

**Architecture:** The CLI fixes the workspace at startup and injects a scan closure into a revisioned server service. The service retains raw scan state only for local transactions, publishes a path-sanitized immutable state through token-protected loopback APIs, and delegates snapshots, drift, fixes, rollback, and ZIP export to focused packages. A bundled TypeScript/SVG dashboard consumes those APIs without external requests or runtime dependencies.

**Tech Stack:** Go 1.24 standard library, Cobra 1.10.1, existing TOML/YAML parsers, TypeScript 5.9.3, esbuild 0.25.11, Playwright 1.56.1, SVG/CSS, Go `embed`, `net/http`, and `archive/zip`.

**Spec:** `docs/superpowers/specs/2026-10-08-harnessscope-v0.2-design.md`

## Global Constraints

- Preserve all existing CLI commands, report schema v1 behavior, exit codes, v0.1 backup manifests, and VERIFIED/PREVIEW evidence boundaries.
- Bind only to `127.0.0.1` or `::1`; no flag may expose the dashboard on a non-loopback address.
- Require a 32-byte random session token on every `/api/v1/*` request; mutation routes also validate Host, Origin, content type, body size, and expected revision.
- Fix the workspace root at process startup; the browser never supplies a filesystem path.
- Keep raw configuration and raw secrets out of model state, responses, logs, snapshots, bundles, fixtures, and browser DOM.
- Continue shipping one offline Go binary with embedded dashboard assets, no telemetry, no cloud service, and no external fonts or scripts.
- Use only synthetic fixtures and credential canaries; never copy a real user configuration into the repository.
- Follow TDD for every behavior: observe the focused test fail before adding production code.
- Run Go tests with `GOCACHE=/private/tmp/harnessscope-go-cache` and `GOMODCACHE=/private/tmp/harnessscope-go-mod` in the current macOS workspace.
- Use `apply_patch` for source edits and preserve unrelated user changes.

## File map

```text
internal/sanitize/report.go              shared public-report path sanitization
internal/analyzers/advanced.go           v0.2 normalized diagnostics
internal/snapshots/store.go              0600 canonical snapshot persistence
internal/snapshots/diff.go               deterministic normalized drift
internal/export/bundle.go                deterministic sanitized ZIP bundles
internal/fixes/history.go                redacted backup summaries
internal/server/types.go                 dashboard/API data contracts
internal/server/service.go               revisioned raw/public state orchestration
internal/server/security.go              loopback token, Host, Origin, headers
internal/server/routes.go                strict JSON HTTP API
internal/server/http.go                  listener lifecycle and embedded assets
internal/server/assets/*                 generated dashboard JS/CSS/HTML
internal/cli/serve.go                    `hscope serve`
internal/cli/snapshot.go                 snapshot save/list/diff CLI
internal/cli/export.go                   offline diagnostic ZIP CLI
web/dashboard/index.html                 accessible dashboard shell
web/dashboard/src/*.ts                   API client, state, navigation, views
web/dashboard/src/*.css                  technical visual system and layout
web/dashboard/tests/*.spec.ts            end-to-end control-center tests
schemas/dashboard-state-v1.schema.json   public state contract
schemas/drift-v1.schema.json             normalized drift contract
schemas/bundle-manifest-v1.schema.json   diagnostic bundle manifest
```

---

### Task 1: Extract Shared Public Sanitization

**Files:**
- Create: `internal/sanitize/report.go`
- Create: `internal/sanitize/report_test.go`
- Modify: `internal/cli/scan.go`
- Modify: `internal/cli/commands_test.go`

**Interfaces:**
- Consumes: `model.ScanResult`, home directory, fixed workspace root.
- Produces: `sanitize.Report(input model.ScanResult, homeDir, workspaceRoot string) (model.ScanResult, error)`.

- [ ] **Step 1: Write the failing sanitization tests**

Create table tests that place the real temporary home and workspace paths in source logical/canonical paths, origins, graph labels, detection executables, run metadata, fix edit targets, replacements, and safe-value path fields. Assert the result contains `~` and `.` but not either absolute prefix, and assert the input remains byte-for-byte unchanged after canonical marshaling.

```go
func TestReportCollapsesEveryPublicPathWithoutMutatingInput(t *testing.T) {
    home := t.TempDir()
    root := filepath.Join(home, "work")
    input := fixtureResultWithPaths(home, root)
    before, _ := model.MarshalCanonical(input)
    got, err := Report(input, home, root)
    if err != nil { t.Fatal(err) }
    encoded, _ := model.MarshalCanonical(got)
    if bytes.Contains(encoded, []byte(home)) || bytes.Contains(encoded, []byte(root)) {
        t.Fatalf("absolute path leaked: %s", encoded)
    }
    after, _ := model.MarshalCanonical(input)
    if !bytes.Equal(before, after) { t.Fatal("input mutated") }
}
```

- [ ] **Step 2: Run the focused test and confirm RED**

Run: `go test ./internal/sanitize -race -v`  
Expected: build failure because `Report` does not exist.

- [ ] **Step 3: Implement deep-copy path sanitization**

Marshal canonically, replace the cleaned workspace prefix before the home prefix, unmarshal into a new result, and reject empty/root replacement prefixes. Keep this behavior in one package so CLI reports, dashboard state, snapshots, and exports cannot diverge.

```go
func Report(input model.ScanResult, homeDir, workspaceRoot string) (model.ScanResult, error) {
    data, err := model.MarshalCanonical(input)
    if err != nil { return model.ScanResult{}, fmt.Errorf("marshal safe report: %w", err) }
    text := string(data)
    if root, err := filepath.Abs(workspaceRoot); err == nil && root != string(filepath.Separator) {
        text = strings.ReplaceAll(text, filepath.Clean(root), ".")
    }
    home := filepath.Clean(homeDir)
    if homeDir != "" && home != "." && home != string(filepath.Separator) {
        text = strings.ReplaceAll(text, home, "~")
    }
    var output model.ScanResult
    if err := json.Unmarshal([]byte(text), &output); err != nil {
        return model.ScanResult{}, fmt.Errorf("decode safe report: %w", err)
    }
    return model.Canonicalize(output), nil
}
```

- [ ] **Step 4: Replace the CLI-local helper and run regression tests**

Delete `sanitizeReportPaths` from `internal/cli/scan.go`, call `sanitize.Report`, and keep terminal/JSON/HTML report behavior unchanged.

Run: `go test ./internal/sanitize ./internal/cli ./internal/report -race -v`  
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/sanitize internal/cli/scan.go internal/cli/commands_test.go
git commit -m "refactor: centralize public report sanitization"
```

---

### Task 2: Add Evidence-Aware Advanced Diagnostics

**Files:**
- Create: `internal/analyzers/advanced.go`
- Create: `internal/analyzers/advanced_test.go`
- Modify: `internal/analyzers/analyzers.go`
- Modify: `internal/cli/runtime.go`
- Modify: `internal/model/model.go`
- Modify: `schemas/report-v1.schema.json`

**Interfaces:**
- Produces: `analyzers.Options{PathEntries []string, ScanRoot string, HomeDir string}`.
- Produces: `analyzers.RunWithOptions(ctx context.Context, input model.Analysis, options analyzers.Options) []model.Finding`.
- Preserves: `analyzers.Run(ctx, input)` as a compatibility wrapper using zero options.

- [ ] **Step 1: Write one positive and one negative test per new rule**

Cover `CLIENT-0001`, `CMD-0001`, `PATH-0002`, `CONFIG-0001`, `SECRET-0001`, and `SOURCE-0003`. Assert exact rule ID, severity, affected clients, origin, and evidence. Include preview nodes and verify behavioral findings are `UNKNOWN`.

```go
func TestAdvancedRulesPreserveWeakestEvidence(t *testing.T) {
    analysis := fixtureAnalysis(
        node("codex", "mcp.shared", model.EvidenceConfirmed, map[string]model.SafeValue{"command": {Display: "missing-tool", Present: true}}),
        node("cursor", "mcp.shared", model.EvidenceUnknown, map[string]model.SafeValue{"command": {Display: "other-tool", Present: true}}),
    )
    got := RunWithOptions(context.Background(), analysis, Options{PathEntries: []string{t.TempDir()}})
    assertFinding(t, got, "CMD-0001", model.EvidenceConfirmed)
    assertFinding(t, got, "CONFIG-0001", model.EvidenceUnknown)
}
```

- [ ] **Step 2: Run and confirm RED**

Run: `go test ./internal/analyzers -race -run 'TestAdvanced' -v`  
Expected: failure because `Options` and advanced rules do not exist.

- [ ] **Step 3: Implement the analyzers as isolated registry functions**

Add option-aware analyzers without changing existing rule output. Resolve bare commands only against captured `PathEntries`; never invoke them. Treat absolute paths under the fixed home/workspace roots as less portable but do not flag the HarnessScope executable itself. For `SECRET-0001`, inspect only `SafeValue.SecretCategory` and origins, never a raw credential value.

```go
type Options struct {
    PathEntries []string
    ScanRoot    string
    HomeDir     string
}

func RunWithOptions(ctx context.Context, input model.Analysis, options Options) []model.Finding {
    findings := Run(ctx, input)
    for _, analyzer := range []func(context.Context, model.Analysis, Options) []model.Finding{
        analyzeCompatibility, analyzeCommands, analyzePortability,
        analyzeDivergence, analyzeSecretPresence, analyzeSymlinkEscape,
    } {
        findings = append(findings, analyzer(ctx, input, options)...)
    }
    sortFindings(findings)
    return findings
}
```

- [ ] **Step 4: Wire runtime options and schema fields**

Call `RunWithOptions` from `Runtime.Scan` with the detected environment and absolute scan root. Do not add raw path-entry values to `RunMetadata` or the report schema.

- [ ] **Step 5: Run analyzers and full regression tests**

Run: `go test ./internal/analyzers ./internal/cli ./internal/model -race -v && go test ./...`  
Expected: PASS with existing rule counts updated only in fixtures that deliberately trigger new rules.

- [ ] **Step 6: Commit**

```bash
git add internal/analyzers internal/cli/runtime.go internal/model/model.go schemas/report-v1.schema.json
git commit -m "feat: add advanced configuration diagnostics"
```

---

### Task 3: Implement Sanitized Snapshot Storage and Drift

**Files:**
- Create: `internal/snapshots/types.go`
- Create: `internal/snapshots/store.go`
- Create: `internal/snapshots/store_test.go`
- Create: `internal/snapshots/diff.go`
- Create: `internal/snapshots/diff_test.go`
- Create: `schemas/drift-v1.schema.json`

**Interfaces:**
- Produces: `snapshots.NewStore(root string, clock func() time.Time) *Store`.
- Produces: `(*Store).Save(name string, result model.ScanResult) (Metadata, error)`.
- Produces: `(*Store).List() ([]Metadata, error)` and `(*Store).Load(name string) (model.ScanResult, error)`.
- Produces: `snapshots.Compare(current, baseline model.ScanResult) snapshots.Diff`.

- [ ] **Step 1: Write failing store tests**

Assert name allowlisting, `0700` root, `0600` files, canonical round trip, stable sorted listing, schema-major rejection, absence of canaries, and refusal to follow a snapshot-name path traversal.

```go
func TestStoreRoundTripUsesPrivatePermissions(t *testing.T) {
    store := NewStore(filepath.Join(t.TempDir(), "snapshots"), fixedClock)
    metadata, err := store.Save("before-upgrade", fixtureSafeResult())
    if err != nil { t.Fatal(err) }
    info, _ := os.Stat(metadata.Path)
    if info.Mode().Perm() != 0o600 { t.Fatalf("mode=%o", info.Mode().Perm()) }
    loaded, err := store.Load("before-upgrade")
    if err != nil { t.Fatal(err) }
    assertCanonicalEqual(t, loaded, fixtureSafeResult())
}
```

- [ ] **Step 2: Run store tests and confirm RED**

Run: `go test ./internal/snapshots -race -run 'TestStore' -v`  
Expected: build failure because the package is absent.

- [ ] **Step 3: Implement the snapshot store**

Validate names with `^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`, write through a same-directory temporary file with sync/rename, and parse with `report.ReadJSON`. Store only inputs already sanitized by the caller; additionally run `secrets.NewRedactor().ScrubText` before persistence as defense in depth.

- [ ] **Step 4: Write failing deterministic drift tests**

Cover added, removed, and changed clients, sources, nodes, findings, and compatibility. Shuffle both inputs and assert identical output. Assert drift entries contain normalized identifiers and summaries, not raw `SafeValue` contents.

```go
func TestCompareIsDeterministicAndValueSafe(t *testing.T) {
    left, right := driftFixtures()
    first := Compare(left, right)
    reverseInputs(left, right)
    second := Compare(left, right)
    if diff := cmpJSON(first, second); diff != "" { t.Fatal(diff) }
    encoded, _ := json.Marshal(first)
    if bytes.Contains(encoded, []byte("HARNESSSCOPE-CANARY")) { t.Fatal("canary leaked") }
}
```

- [ ] **Step 5: Implement drift types and comparison**

Use the exact public contract:

```go
const SchemaVersion = "1.0.0"
type Change struct {
    Kind       string   `json:"kind"`       // ADDED, REMOVED, CHANGED
    EntityType string   `json:"entity_type"` // CLIENT, SOURCE, NODE, FINDING, COMPATIBILITY
    ID         string   `json:"id"`
    Summary    string   `json:"summary"`
    Clients    []string `json:"clients,omitempty"`
}
type Diff struct {
    SchemaVersion string   `json:"schema_version"`
    Baseline      string   `json:"baseline,omitempty"`
    Changes       []Change `json:"changes"`
}
```

Compute fingerprints from allowlisted normalized fields; exclude timestamps, absolute paths, and secret displays. Sort by entity type, ID, then kind.

- [ ] **Step 6: Add and parse the strict drift schema**

Use JSON Schema 2020-12 with `additionalProperties: false` on every stable object and enums matching the Go constants.

- [ ] **Step 7: Run and commit**

Run: `go test ./internal/snapshots -race -v && python3 -m json.tool schemas/drift-v1.schema.json >/dev/null`  
Expected: PASS.

```bash
git add internal/snapshots schemas/drift-v1.schema.json
git commit -m "feat: add sanitized snapshots and drift"
```

---

### Task 4: Add Redacted Backup History and Deterministic Bundle Export

**Files:**
- Create: `internal/fixes/history.go`
- Create: `internal/fixes/history_test.go`
- Create: `internal/export/bundle.go`
- Create: `internal/export/bundle_test.go`
- Create: `schemas/bundle-manifest-v1.schema.json`

**Interfaces:**
- Produces: `fixes.ListBackups(root string) ([]fixes.BackupSummary, error)`.
- Produces: `export.New(clock func() time.Time) *Exporter`.
- Produces: `(*Exporter).Write(ctx context.Context, output io.Writer, input export.Input) (export.Manifest, error)`.

- [ ] **Step 1: Write failing backup-history tests**

Create valid, malformed, and traversal-shaped backup directories. Assert only valid manifests become summaries and no target paths or backup bytes are returned.

```go
type BackupSummary struct {
    ID        string `json:"id"`
    CreatedAt string `json:"created_at"`
    FileCount int    `json:"file_count"`
}
```

- [ ] **Step 2: Implement strict backup enumeration**

Reuse the timestamp-random ID grammar, open only `<root>/<validated-id>/manifest.json`, verify the manifest ID, count entries, and derive time from the ID. Sort newest first. A malformed unrelated directory is ignored; a selected malformed manifest still fails rollback through existing behavior.

- [ ] **Step 3: Write failing bundle tests**

Use an injected clock and assert two identical inputs produce identical ZIP bytes. Assert exact members, lexicographic member order, `0600`-equivalent ZIP modes, manifest SHA-256 verification, HTML offline markers, and rejection when any member contains `HARNESSSCOPE-CANARY`, a raw home path, or credential pattern.

```go
type Input struct {
    ToolVersion string
    Report      model.ScanResult
    Drift       *snapshots.Diff
}
```

- [ ] **Step 4: Implement deterministic ZIP export**

Build report JSON through `report.WriteJSON`, render HTML internally through `report.WriteHTML`, serialize optional drift, then hash each member. Use a fixed injected time for every ZIP header, `zip.Store` for reproducible bytes, and write `manifest.json` last.

```go
type Manifest struct {
    SchemaVersion string       `json:"schema_version"`
    ToolVersion   string       `json:"tool_version"`
    GeneratedAt   string       `json:"generated_at"`
    Clients       []ClientTier `json:"clients"`
    Members       []MemberHash `json:"members"`
}
```

Before returning, scan all uncompressed member bytes with the common redactor and explicitly reject the repository canary marker. `README.txt` explains that the bundle is sanitized but still requires human review before public upload.

- [ ] **Step 5: Add strict manifest schema and run tests**

Run: `go test ./internal/fixes ./internal/export -race -v && python3 -m json.tool schemas/bundle-manifest-v1.schema.json >/dev/null`  
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/fixes/history.go internal/fixes/history_test.go internal/export schemas/bundle-manifest-v1.schema.json
git commit -m "feat: add backup history and diagnostic bundles"
```

---

### Task 5: Build the Revisioned Dashboard Service

**Files:**
- Create: `internal/server/types.go`
- Create: `internal/server/service.go`
- Create: `internal/server/service_test.go`
- Create: `schemas/dashboard-state-v1.schema.json`

**Interfaces:**
- Consumes: injected scan closure, fixed workspace/home/app-data paths, snapshot store, and clock.
- Produces: `server.NewService(config server.ServiceConfig) (*server.Service, error)`.
- Produces: `State`, `Rescan`, `PlanFixes`, `ApplyFixes`, `Rollback`, `SaveSnapshot`, `CompareSnapshot`, and `Export` methods.

- [ ] **Step 1: Define and test immutable public state**

Use this contract:

```go
type ScanFunc func(context.Context) (model.ScanResult, error)
type DashboardState struct {
    SchemaVersion string                 `json:"schema_version"`
    Revision      uint64                 `json:"revision"`
    ScannedAt     string                 `json:"scanned_at"`
    Workspace     string                 `json:"workspace"`
    Result        model.ScanResult       `json:"result"`
    FixPlans      []model.FixPlan        `json:"fix_plans"`
    Backups       []fixes.BackupSummary  `json:"backups"`
    Drift         *snapshots.Diff        `json:"drift,omitempty"`
}
type ServiceConfig struct {
    Workspace  string
    HomeDir    string
    AppDataDir string
    Scan       ScanFunc
    Clock      func() time.Time
}
```

Test that initial construction scans once, revision starts at 1, published paths are sanitized, and mutating a caller-owned returned struct cannot modify subsequent `State()` results.

- [ ] **Step 2: Run service tests and confirm RED**

Run: `go test ./internal/server -race -run 'TestService' -v`  
Expected: build failure because the service package does not exist.

- [ ] **Step 3: Implement raw/public state separation**

Keep `raw model.ScanResult` private under `sync.RWMutex`. Build public state with `sanitize.Report`, `fixes.Plan`, and `fixes.ListBackups`; deep-copy state through canonical JSON at publication and read boundaries. Never include the session token in service state.

- [ ] **Step 4: Add revision and failure semantics tests**

Test concurrent `State()` and `Rescan()` under `-race`; a failed rescan preserves the prior state/revision; a successful rescan increments exactly once; stale mutation revisions return `ErrStaleRevision` before filesystem access.

- [ ] **Step 5: Implement service operations**

`ApplyFixes` must rescan/replan at the expected revision, accept named SAFE IDs only, run existing `fixes.Apply` with rescan verification, and publish once after all selected transactions succeed. If a later selected plan fails, roll back transactions already applied in that request in reverse order. `Rollback` accepts one validated backup ID and publishes after successful rescan.

Snapshot and export methods always consume the current public state. `CompareSnapshot` attaches the resulting diff through a new revision because it changes visible state but not the workspace.

- [ ] **Step 6: Add strict dashboard-state schema**

Reference existing report/fix definitions where possible and use `additionalProperties: false`. Require revision, timestamp, workspace display path, result, fix plans, and backups.

- [ ] **Step 7: Run and commit**

Run: `go test ./internal/server ./internal/fixes ./internal/snapshots ./internal/export -race -v`  
Expected: PASS.

```bash
git add internal/server/types.go internal/server/service.go internal/server/service_test.go schemas/dashboard-state-v1.schema.json
git commit -m "feat: add revisioned dashboard service"
```

---

### Task 6: Enforce Loopback Session Security

**Files:**
- Create: `internal/server/security.go`
- Create: `internal/server/security_test.go`

**Interfaces:**
- Produces: `server.NewSession(random io.Reader) (Session, error)`.
- Produces: `server.Session.Protect(next http.Handler) http.Handler`.
- Produces: `server.ValidateListenHost(host string) error`.

- [ ] **Step 1: Write failing security matrix tests**

Cover 32-byte entropy, base64url token encoding, wrong/missing token, valid loopback Host with active port, malicious Host, remote bind request, absent/foreign Origin for mutation, read requests without Origin, JSON content type, 1 MiB body limit, and required headers on both success and error.

```go
func TestSessionRejectsCrossOriginMutation(t *testing.T) {
    session := fixedSession(t)
    request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8123/api/v1/rescan", strings.NewReader(`{}`))
    request.Host = "127.0.0.1:8123"
    request.Header.Set("Origin", "https://evil.example")
    request.Header.Set("X-HarnessScope-Token", session.Token())
    response := httptest.NewRecorder()
    session.Protect(okHandler()).ServeHTTP(response, request)
    if response.Code != http.StatusForbidden { t.Fatalf("code=%d", response.Code) }
}
```

- [ ] **Step 2: Run and confirm RED**

Run: `go test ./internal/server -race -run 'TestSession|TestValidateListenHost' -v`  
Expected: build failure for missing security functions.

- [ ] **Step 3: Implement the guard**

Generate 32 bytes with `crypto/rand`, encode with `base64.RawURLEncoding`, compare tokens with `subtle.ConstantTimeCompare`, parse Host using `net.SplitHostPort`, require `net.IP.IsLoopback`, and accept Origins only when scheme is `http` and host exactly matches the active listener.

Set `Content-Security-Policy: default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'`, `X-Content-Type-Options`, `Referrer-Policy`, and no-store headers.

- [ ] **Step 4: Run and commit**

Run: `go test ./internal/server -race -run 'TestSession|TestValidateListenHost' -v`  
Expected: PASS.

```bash
git add internal/server/security.go internal/server/security_test.go
git commit -m "feat: secure local dashboard sessions"
```

---

### Task 7: Implement Strict JSON API Routes

**Files:**
- Create: `internal/server/routes.go`
- Create: `internal/server/routes_test.go`
- Modify: `internal/server/types.go`

**Interfaces:**
- Consumes: `*server.Service` and `server.Session`.
- Produces: `server.NewHandler(service *Service, session Session) http.Handler`.
- Produces: stable `APIError{Code string, Message string, Details map[string]string}` envelope.

- [ ] **Step 1: Write failing route contract tests**

Create `httptest` cases for every route in the spec. Verify method restrictions, exact status codes, unknown-field rejection, body limits, content types, stale revision 409, missing item 404, unsafe fix 422, redacted 500, download headers, and authentication on reads and writes.

```go
func decodeStrict[T any](writer http.ResponseWriter, request *http.Request, target *T) error {
    decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 1<<20))
    decoder.DisallowUnknownFields()
    if err := decoder.Decode(target); err != nil { return err }
    if decoder.Decode(&struct{}{}) != io.EOF { return errors.New("multiple JSON values") }
    return nil
}
```

- [ ] **Step 2: Run and confirm RED**

Run: `go test ./internal/server -race -run 'TestRoutes' -v`  
Expected: failure because routes are missing.

- [ ] **Step 3: Implement read routes**

Implement state, explain, compare, backups, and snapshots with Go 1.24 `http.ServeMux` method patterns. Explain resolves exact IDs only. Compare accepts registered client IDs already present in state. Encode canonical ordering through the service.

- [ ] **Step 4: Implement mutation routes**

Define exact bodies:

```go
type RevisionRequest struct { Revision uint64 `json:"revision"` }
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
```

Rescan uses `RevisionRequest`; export accepts revision and optional active baseline, then streams `application/zip` with a constant safe filename.

- [ ] **Step 5: Implement error mapping and redaction**

Map sentinel errors with `errors.Is`; scrub every returned message through the common redactor. Internal errors receive a generic public message and are returned to an injected server logger only after redaction.

- [ ] **Step 6: Run and commit**

Run: `go test ./internal/server -race -v`  
Expected: PASS.

```bash
git add internal/server/routes.go internal/server/routes_test.go internal/server/types.go
git commit -m "feat: add local dashboard API"
```

---

### Task 8: Add Listener Lifecycle, Embedded Assets, and `hscope serve`

**Files:**
- Create: `internal/server/http.go`
- Create: `internal/server/http_test.go`
- Create: `internal/server/assets/index.html`
- Create: `internal/server/assets/dashboard.js`
- Create: `internal/server/assets/dashboard.css`
- Create: `internal/cli/serve.go`
- Create: `internal/cli/serve_test.go`
- Modify: `internal/cli/root.go`
- Modify: `internal/cli/runtime.go`
- Modify: `internal/cli/root_test.go`

**Interfaces:**
- Produces: `server.Start(ctx context.Context, config server.HTTPConfig) (*server.Instance, error)`.
- Produces: `(*Instance).URL() string`, `(*Instance).Wait() error`, and `(*Instance).Close(ctx) error`.
- Adds: `hscope serve [path] --client ... --port 0 --open`.

- [ ] **Step 1: Write failing listener tests**

Assert port 0 chooses an available loopback port, configured port binds only loopback, readiness waits for initial state, token appears only in returned launch URL fragment, static assets work without token, APIs reject missing token, cancellation shuts down, and a non-loopback host is rejected.

- [ ] **Step 2: Implement embedded HTTP lifecycle**

Embed `assets/*`, serve `/` and `/assets/*` with explicit MIME types, route `/api/v1/*` through the security middleware, and return 404 for every other path. Use `net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))` and a bounded five-second shutdown context.

```go
type HTTPConfig struct {
    Port    int
    Service *Service
    Random  io.Reader
    Logger  *log.Logger
}

func Start(ctx context.Context, config HTTPConfig) (*Instance, error) {
    listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(config.Port)))
    if err != nil { return nil, err }
    session, err := NewSession(config.Random)
    if err != nil { listener.Close(); return nil, err }
    instance := newInstance(listener, session, config.Service, config.Logger)
    go instance.serve(ctx)
    return instance, nil
}
```

- [ ] **Step 3: Write failing CLI lifecycle tests**

Inject a fake `StartServer` function into `Runtime` so tests do not open a real port. Assert fixed path/client scan options, printed one-time URL, `--open` behavior, context cancellation, and no report files written.

- [ ] **Step 4: Wire the command**

Extend `Runtime` with a typed server starter and tool version. `newServeCommand` resolves the workspace once, constructs a scan closure using existing `Runtime.Scan`, creates `Service`, starts the server, optionally opens its URL, prints the URL once, and blocks on `Wait` until cancellation or server failure.

```go
scan := func(ctx context.Context) (model.ScanResult, error) {
    return runtime.Scan(ctx, ScanOptions{CWD: absolutePath, Clients: clients, NoReport: true})
}
service, err := server.NewService(server.ServiceConfig{
    Workspace: absolutePath, HomeDir: runtime.Environment.HomeDir,
    AppDataDir: runtime.Environment.AppDataDir, Scan: scan, Clock: time.Now,
})
```

- [ ] **Step 5: Run and commit**

Run: `go test ./internal/server ./internal/cli -race -v && go test ./...`  
Expected: PASS and root help lists `serve`.

```bash
git add internal/server/http.go internal/server/http_test.go internal/server/assets internal/cli/serve.go internal/cli/serve_test.go internal/cli/root.go internal/cli/runtime.go internal/cli/root_test.go
git commit -m "feat: serve the local control center"
```

---

### Task 9: Build the Technology-Styled Dashboard Foundation

**Files:**
- Create: `web/dashboard/index.html`
- Create: `web/dashboard/src/api.ts`
- Create: `web/dashboard/src/types.ts`
- Create: `web/dashboard/src/store.ts`
- Create: `web/dashboard/src/app.ts`
- Create: `web/dashboard/src/dashboard.css`
- Create: `web/dashboard/tests/dashboard.spec.ts`
- Create: `web/dashboard/tests/fixture-state.json`
- Create: `web/dashboard/playwright.config.ts`
- Modify: `web/package.json`
- Modify: `web/tsconfig.json`
- Modify: `.gitignore`

**Interfaces:**
- Consumes: dashboard state/API schemas from Tasks 5–7.
- Produces: reproducible embedded `internal/server/assets/index.html`, `dashboard.js`, and `dashboard.css`.

- [ ] **Step 1: Write failing browser shell tests**

Start a synthetic authenticated API fixture and assert token fragment removal, API header use, no non-file/non-loopback requests, navigation landmarks, visible client tiers, risk cards, keyboard traversal, 768 px usability, reduced-motion behavior, and absence of `innerHTML` assignments in source.

- [ ] **Step 2: Define TypeScript contracts and strict API client**

Mirror stable JSON fields only. `APIClient` reads the fragment token once, clears the fragment, sends same-origin requests with the token, rejects non-JSON error envelopes, and exposes typed methods matching every API route.

```ts
export class APIClient {
  constructor(private readonly token: string, private readonly base = "") {}
  async state(): Promise<DashboardState> { return this.request("/api/v1/state"); }
  async rescan(revision: number): Promise<DashboardState> {
    return this.request("/api/v1/rescan", { method: "POST", body: { revision } });
  }
}
```

- [ ] **Step 3: Implement store and accessible shell**

Use a small observer store with `state`, `route`, `selection`, `busy`, and `error`; no framework dependency. Create DOM elements through `document.createElement`, `textContent`, and attribute setters. The shell contains left rail, top bar, main view, right inspector, toast region, and modal root.

```ts
export type Listener = (state: UIState) => void;
export class Store {
  private listeners = new Set<Listener>();
  constructor(private value: UIState) {}
  get(): Readonly<UIState> { return this.value; }
  update(change: Partial<UIState>): void {
    this.value = { ...this.value, ...change };
    for (const listener of this.listeners) listener(this.value);
  }
  subscribe(listener: Listener): () => void {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  }
}
```

- [ ] **Step 4: Implement the visual system**

Use CSS custom properties for navy surfaces, cyan/purple accents, severity colors, focus rings, spacing, and typography. Add a pseudo-element grid background, restrained box shadows, status pulses, and transitions behind `@media (prefers-reduced-motion: no-preference)`. Use system fonts and never fetch a font.

- [ ] **Step 5: Add reproducible build scripts**

Add `build:dashboard`, keep `build:report`, and make `build` run both. Copy HTML verbatim and bundle/minify TS/CSS with esbuild into `internal/server/assets`.

- [ ] **Step 6: Run foundation tests and commit**

Run: `npm --prefix web ci && npm --prefix web run build && (cd web && npx tsc --noEmit) && npm --prefix web run test:dashboard`  
Expected: PASS and `git diff --exit-code` after a second build.

```bash
git add web internal/server/assets .gitignore
git commit -m "feat: add local control center interface"
```

---

### Task 10: Implement Graph, Findings, Compare, and Inspector Views

**Files:**
- Create: `web/dashboard/src/graph.ts`
- Create: `web/dashboard/src/findings.ts`
- Create: `web/dashboard/src/compare.ts`
- Create: `web/dashboard/src/inspector.ts`
- Create: `web/dashboard/src/views.ts`
- Modify: `web/dashboard/src/app.ts`
- Modify: `web/dashboard/src/dashboard.css`
- Modify: `web/dashboard/tests/dashboard.spec.ts`
- Create: `web/dashboard/tests/__snapshots__/overview.png`

**Interfaces:**
- Produces: pure `renderGraph`, `renderFindings`, `renderCompare`, and `renderInspector` functions driven by store state.

- [ ] **Step 1: Write failing interaction tests**

Assert graph nodes/edges render from fixture state; wheel/button zoom and pointer pan update SVG transforms; search and client/scope/evidence filters hide the correct nodes; arrow keys move selection; Enter opens inspector; a finding focuses its graph node; compare reports missing/divergent entries; all controls have accessible names.

- [ ] **Step 2: Implement deterministic SVG graph layout**

Group nodes into client/source/config columns with deterministic x positions and stable y order by node ID. Render edge types with distinct dash patterns and evidence opacity. Limit labels but preserve full text in accessible descriptions and inspector data. Use an SVG `g` transform for pan/zoom; clamp scale to 0.5–2.5.

```ts
export function layout(nodes: ConfigNode[]): PositionedNode[] {
  const columns = new Map<NodeType, number>([["CLIENT", 0], ["SOURCE", 1]]);
  const sorted = [...nodes].sort((a, b) => a.id.localeCompare(b.id));
  const counts = new Map<number, number>();
  return sorted.map((node) => {
    const column = columns.get(node.type) ?? 2;
    const row = counts.get(column) ?? 0;
    counts.set(column, row + 1);
    return { ...node, x: 120 + column * 300, y: 70 + row * 76 };
  });
}
```

- [ ] **Step 3: Implement findings and compare**

Findings filters combine severity, evidence, client, and text. Compare normalizes by `type + display_name`, showing present, missing, or divergent without exposing secret values.

- [ ] **Step 4: Implement origin inspector**

Show node type, client, evidence badge, load condition, redacted attributes, and ordered origins. Source paths must already be `.`/`~`; add a browser assertion that fails on `/Users/`, `/home/`, or fixture canaries.

- [ ] **Step 5: Run browser and visual tests**

Run: `npm --prefix web run build && npm --prefix web run test:dashboard`  
Expected: all functional tests pass. Generate/compare the 1440x1000 snapshot only on Linux CI; locally inspect a captured screenshot without replacing the committed Linux baseline.

- [ ] **Step 6: Commit**

```bash
git add web/dashboard internal/server/assets
git commit -m "feat: visualize configuration provenance"
```

---

### Task 11: Add Browser SAFE Fixes and Rollback

**Files:**
- Create: `web/dashboard/src/fixes.ts`
- Modify: `web/dashboard/src/api.ts`
- Modify: `web/dashboard/src/app.ts`
- Modify: `web/dashboard/src/dashboard.css`
- Create: `web/dashboard/tests/fixes.spec.ts`
- Modify: `internal/server/service_test.go`
- Modify: `internal/server/routes_test.go`

**Interfaces:**
- Consumes: fix-plan/apply/rollback endpoints and expected revision.
- Produces: Fix Center UI with confirmation modal, result feedback, backup history, and rollback confirmation.

- [ ] **Step 1: Write failing HTTP round-trip tests**

Use a temporary duplicate instruction source. Through authenticated HTTP calls, verify plan output, stale-revision refusal, SAFE-only selection, apply, rescan, backup summary, rollback, and restored bytes. Assert no canary or raw source content reaches responses.

- [ ] **Step 2: Close transaction gaps exposed by HTTP tests**

If multiple selected fixes touch the same file, replan after each successful transaction or group compatible edits so every source hash is current. Record applied backup IDs and reverse-rollback them if a later selected fix fails. Keep this orchestration in `Service`, not route handlers.

- [ ] **Step 3: Write failing browser tests**

Assert dry plan cards show ID/risk/redacted patch; apply is disabled until at least one SAFE plan is selected; confirmation states file count and backup behavior; stale 409 triggers a rescan prompt; success updates revision/history; rollback requires named confirmation and refreshes state.

- [ ] **Step 4: Implement Fix Center**

Render patches through `textContent`. REVIEW/MANUAL cards explain why browser apply is unavailable. The modal traps focus, Escape cancels, and submit remains disabled while a request is active.

```ts
async function applySelected(store: Store, api: APIClient, ids: string[]): Promise<void> {
  const current = store.get();
  store.update({ busy: true, error: undefined });
  try {
    const next = await api.applyFixes(current.dashboard.revision, ids);
    store.update({ dashboard: next });
  } catch (error) {
    store.update({ error: toSafeMessage(error) });
  } finally {
    store.update({ busy: false });
  }
}
```

- [ ] **Step 5: Run and commit**

Run: `go test ./internal/server ./internal/fixes -race -v && npm --prefix web run build && npm --prefix web run test:dashboard`  
Expected: PASS.

```bash
git add internal/server web/dashboard internal/server/assets
git commit -m "feat: manage safe fixes from the dashboard"
```

---

### Task 12: Add Drift and Diagnostic Export Views plus CLI Commands

**Files:**
- Create: `web/dashboard/src/drift.ts`
- Create: `web/dashboard/src/export.ts`
- Modify: `web/dashboard/src/api.ts`
- Modify: `web/dashboard/src/app.ts`
- Modify: `web/dashboard/src/dashboard.css`
- Create: `web/dashboard/tests/drift-export.spec.ts`
- Create: `internal/cli/snapshot.go`
- Create: `internal/cli/snapshot_test.go`
- Create: `internal/cli/export.go`
- Create: `internal/cli/export_test.go`
- Modify: `internal/cli/root.go`
- Modify: `internal/cli/root_test.go`

**Interfaces:**
- Adds: `hscope snapshot save <name> [path]`, `hscope snapshot list`, `hscope snapshot diff <name> [path]`.
- Adds: `hscope export [path] --output diagnostic.zip [--baseline name]`.

- [ ] **Step 1: Write failing dashboard tests**

Assert baseline list/save, deterministic drift groups, empty-drift state, stale revision behavior, ZIP download name/type, and a visible pre-publication privacy warning.

- [ ] **Step 2: Implement Drift and Export views**

Group changes by added/removed/changed and entity type, link node/finding IDs back to existing views, and display no raw before/after secret values. Create downloads from a Blob and immediately revoke object URLs.

```ts
export async function downloadBundle(api: APIClient, revision: number): Promise<void> {
  const blob = await api.exportBundle(revision);
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = "harnessscope-diagnostic.zip";
  anchor.click();
  URL.revokeObjectURL(url);
}
```

- [ ] **Step 3: Write failing CLI tests**

Inject temporary app data and fixture runtime. Assert snapshot permissions/listing/diff JSON, export exact ZIP members, overwrite refusal unless `--force` is explicit, stdout paths, exit codes, and no workspace pollution.

- [ ] **Step 4: Implement additive CLI commands**

All commands call `Runtime.Scan`, then `sanitize.Report`, and delegate to the snapshot/export packages. `export` opens the output with `O_CREATE|O_EXCL` by default; `--force` uses atomic same-directory replacement rather than truncating the destination in place.

```go
func writeBundleAtomically(target string, force bool, write func(io.Writer) error) error {
    if !force {
        file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
        if err != nil { return err }
        return closeAfter(file, write(file))
    }
    temporary, err := os.CreateTemp(filepath.Dir(target), ".hscope-export-*")
    if err != nil { return err }
    defer os.Remove(temporary.Name())
    if err := temporary.Chmod(0o600); err != nil { return err }
    if err := write(temporary); err != nil { temporary.Close(); return err }
    if err := temporary.Sync(); err != nil { temporary.Close(); return err }
    if err := temporary.Close(); err != nil { return err }
    return os.Rename(temporary.Name(), target)
}
```

- [ ] **Step 5: Run and commit**

Run: `go test ./internal/cli ./internal/snapshots ./internal/export -race -v && npm --prefix web run build && npm --prefix web run test:dashboard`  
Expected: PASS and root help lists `serve`, `snapshot`, and `export`.

```bash
git add internal/cli web/dashboard internal/server/assets
git commit -m "feat: expose drift and diagnostic export workflows"
```

---

### Task 13: Upgrade the Public Demo, Documentation, and Release Gates

**Files:**
- Modify: `demo/run.sh`
- Create: `demo/dashboard.sh`
- Modify: `demo/conflicted-workspace/EXPECTED.md`
- Modify: `README.md`
- Modify: `README.zh-CN.md`
- Modify: `docs/architecture.md`
- Modify: `SECURITY.md`
- Modify: `THIRD_PARTY_NOTICES.md`
- Modify: `scripts/check-third-party-notices.sh`
- Modify: `scripts/smoke-release.sh`
- Modify: `scripts/build-release.sh`
- Modify: `.github/workflows/ci.yml`
- Modify: `.github/workflows/release.yml`
- Create: `docs/assets/dashboard-overview.png`

**Interfaces:**
- Produces: a reproducible synthetic dashboard demo and v0.2 release archives for darwin/linux × amd64/arm64.

- [ ] **Step 1: Extend the synthetic demo assertions**

Add fixtures that trigger each v0.2 analyzer without changing real machine state. `demo/dashboard.sh` builds the binary, starts on port 0, prints the authenticated URL once, and terminates cleanly on Ctrl-C. Keep `demo/run.sh` noninteractive and add snapshot/drift/export assertions.

- [ ] **Step 2: Update public documentation from verified behavior**

Document `serve`, screenshots, security boundary, all compatibility tiers, fix confirmation, snapshot commands, diagnostic ZIP contents, and limitations. Generate `docs/assets/dashboard-overview.png` from the committed Playwright fixture only after visual inspection. Do not claim remote access, automatic REVIEW fixes, or semantic analysis.

- [ ] **Step 3: Re-audit dependency licenses**

If package locks changed, enumerate the exact Go and npm graph, verify license compatibility, update notice versions/upstream links, update recorded lock hashes, and make `check-third-party-notices.sh` fail on either stale hash.

- [ ] **Step 4: Harden CI**

Run Go normal/race/vet, dashboard and report TypeScript builds, reproducibility diff, both Playwright suites, repository canary policy, demo assertions, strict schema parse, shell syntax, dashboard smoke, and release matrix. Linux alone performs visual comparison.

- [ ] **Step 5: Run the complete local release gate**

```bash
gofmt -w cmd internal
GOCACHE=/private/tmp/harnessscope-go-cache GOMODCACHE=/private/tmp/harnessscope-go-mod go test ./...
GOCACHE=/private/tmp/harnessscope-go-cache GOMODCACHE=/private/tmp/harnessscope-go-mod go test -race ./...
GOCACHE=/private/tmp/harnessscope-go-cache GOMODCACHE=/private/tmp/harnessscope-go-mod go vet ./...
npm --prefix web ci
npm --prefix web run build
(cd web && npx tsc --noEmit)
PLAYWRIGHT_BROWSERS_PATH=/private/tmp/harnessscope-playwright npm --prefix web run test:visual
PLAYWRIGHT_BROWSERS_PATH=/private/tmp/harnessscope-playwright npm --prefix web run test:dashboard
GOCACHE=/private/tmp/harnessscope-go-cache GOMODCACHE=/private/tmp/harnessscope-go-mod go build -trimpath -o bin/hscope ./cmd/hscope
./scripts/smoke-release.sh ./bin/hscope
./scripts/check-third-party-notices.sh
./demo/run.sh
git diff --check
```

Expected: every command passes; generated reports and bundles contain neither a real machine path nor `HARNESSSCOPE-CANARY`.

- [ ] **Step 6: Cross-build and verify all archives**

Run `./scripts/build-release.sh v0.2.0 <fresh-temp-directory>`, change into that directory, run `shasum -a 256 -c SHA256SUMS`, and smoke-test the compatible archive after fresh extraction.

- [ ] **Step 7: Commit**

```bash
git add demo docs README.md README.zh-CN.md SECURITY.md THIRD_PARTY_NOTICES.md scripts .github web internal/server/assets
git commit -m "docs: prepare HarnessScope v0.2 release"
```

---

## Final verification checklist

- [ ] Every acceptance criterion in the v0.2 design has a direct automated test or a documented manual visual check.
- [ ] `git status --short` is empty and `git diff --check` passes.
- [ ] The dashboard is reachable only on loopback and every API route rejects an absent token.
- [ ] Mutation tests prove stale revisions, invalid origins, non-SAFE plans, and concurrent source changes cannot modify files.
- [ ] Browser SAFE fix, backup, rollback, snapshot, drift, and ZIP export round trips pass on synthetic fixtures.
- [ ] Terminal, static report, dashboard state, snapshots, logs, errors, and bundles contain no canary or real machine path.
- [ ] Existing v0.1 CLI, report, and rollback behavior remains green.
- [ ] Four release archives verify against `SHA256SUMS`.
- [ ] GitHub repository creation, push, and release remain separate user-authorized actions after local gates pass.
