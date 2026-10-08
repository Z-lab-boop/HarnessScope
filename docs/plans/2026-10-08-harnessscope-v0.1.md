# HarnessScope v0.1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build and release a local, offline-first `hscope` CLI that explains the effective AI coding-assistant configuration for a working directory, with verified Codex and Claude Code adapters, preview Cursor and OpenCode adapters, evidence-bearing findings, secret-safe JSON/HTML reports, and reversible safe fixes.

**Architecture:** A Go 1.24+ single binary owns discovery, client adapters, normalized provenance graphs, analyzers, fix transactions, and CLI commands. A small TypeScript report frontend is compiled during development into embedded CSS/JavaScript assets so the shipped report remains one reproducible offline HTML file. Client-specific behavior remains isolated behind `ClientAdapter`, while shared analyzers consume only normalized models.

**Tech Stack:** Go 1.24+, standard library, `github.com/spf13/cobra`, `github.com/pelletier/go-toml/v2`, `gopkg.in/yaml.v3`, Go `embed`, TypeScript 5.x, esbuild, GitHub Actions, shell release smoke tests.

**Spec:** `docs/design/2026-10-08-harnessscope-design.md` copied from `/Users/zzz/Documents/ChatGPT/大创项目/docs/superpowers/specs/2026-10-08-harnessscope-design.md` during repository bootstrap.

## Global Constraints

- Work in the independent repository `/Users/zzz/Documents/ChatGPT/HarnessScope`; do not add product code to the innovation-project repository.
- Use Go 1.24+ for the product and Node.js 20+ only for reproducible report-asset builds and browser tests; the shipped CLI has no Node.js runtime dependency.
- The core scanner has no network code, telemetry, account dependency, or model API dependency.
- Support macOS and Linux on amd64 and arm64; Windows is deferred.
- Codex and Claude Code are `VERIFIED`; Cursor and OpenCode are visibly `PREVIEW` and cannot make unsupported precedence claims.
- Every finding carries a rule ID, severity, evidence status, source location, reason, impact, and remediation.
- Secret values must be redacted immediately after parsing and must never enter reports, logs, errors, backup metadata, or fixtures.
- `scan` is read-only and writes reports only to per-user application data unless explicit output paths are supplied.
- `fix` defaults to dry-run; mutation requires `--apply`, a backup, atomic writes, rescan verification, and rollback on failure.
- JSON output is schema-versioned and deterministic apart from a separate optional run-metadata section.
- Use TDD for each behavior and keep commits scoped to one task.

---

## File Map

All paths below are relative to `/Users/zzz/Documents/ChatGPT/HarnessScope`.

```text
cmd/hscope/main.go                         process entry point
internal/cli/root.go                       Cobra root command and exit handling
internal/cli/scan.go                       scan command orchestration
internal/cli/explain.go                    provenance explanation command
internal/cli/compare.go                    cross-client comparison command
internal/cli/report.go                     render saved JSON command
internal/cli/fix.go                        dry-run/apply command
internal/cli/rollback.go                   backup restoration command
internal/model/model.go                    normalized domain types
internal/model/canonical.go                deterministic ordering and JSON
internal/secrets/redact.go                 early secret classification/redaction
internal/discovery/environment.go           executable, version, app-data, PATH helpers
internal/discovery/sources.go               bounded source and symlink discovery
internal/adapters/adapter.go                adapter contract and registry
internal/adapters/codex/adapter.go          verified Codex parsing/resolution
internal/adapters/claude/adapter.go         verified Claude Code parsing/resolution
internal/adapters/cursor/adapter.go         preview Cursor syntax/source inspection
internal/adapters/opencode/adapter.go       preview OpenCode syntax/source inspection
internal/formats/jsonc.go                   bounded JSON-with-comments parsing
internal/resolver/graph.go                  graph assembly and stable identifiers
internal/analyzers/analyzers.go             analyzer registry
internal/analyzers/structural.go            parse/source/path/override checks
internal/analyzers/context.go               deterministic context-cost estimates
internal/fixes/plan.go                      safe fix classification and planning
internal/fixes/transaction.go               backup, atomic apply, verify, rollback
internal/report/terminal.go                 compact terminal output
internal/report/json.go                     stable JSON report codec
internal/report/html.go                     self-contained offline HTML renderer
web/package.json                            pinned frontend build dependencies
web/src/report.ts                           typed filtering/navigation behavior
web/src/report.css                          report styles
web/tests/report.spec.ts                    offline/accessibility/visual browser tests
internal/report/assets/report.css           compiled embedded report styles
internal/report/assets/report.js            compiled embedded report behavior
schemas/report-v1.schema.json               public report schema
schemas/fix-plan-v1.schema.json             public fix-plan schema
fixtures/                                   synthetic client and security fixtures
demo/conflicted-workspace/                  deliberately conflicted public demo
demo/run.sh                                 repeatable terminal-to-graph demo
docs/design/                                approved design specification
docs/architecture.md                        contributor architecture guide
README.md                                   English product/install/usage guide
README.zh-CN.md                             complete Chinese guide
SECURITY.md                                 private vulnerability reporting policy
CONTRIBUTING.md                             development and fixture rules
.github/workflows/ci.yml                    test/race/vet/build matrix
.github/workflows/release.yml               tagged archive and checksum build
scripts/smoke-release.sh                    clean-directory artifact smoke test
```

---

### Task 1: Bootstrap the Independent Repository and CLI Contract

**Files:**
- Create: `go.mod`
- Create: `cmd/hscope/main.go`
- Create: `internal/cli/root.go`
- Create: `internal/cli/root_test.go`
- Create: `.gitignore`
- Create: `LICENSE`
- Create: `docs/design/2026-10-08-harnessscope-design.md`
- Create: `docs/plans/2026-10-08-harnessscope-v0.1.md`

**Interfaces:**
- Produces: `cli.Execute(ctx context.Context, args []string, stdout, stderr io.Writer) int`
- Produces: stable command names `scan`, `explain`, `compare`, `report`, `fix`, and `rollback`

- [ ] **Step 1: Create and initialize the isolated repository**

Run:

```bash
mkdir -p /Users/zzz/Documents/ChatGPT/HarnessScope
cd /Users/zzz/Documents/ChatGPT/HarnessScope
git init -b main
go mod init github.com/Z-lab-boop/harnessscope
```

Expected: an empty Git repository and a valid `go.mod`.

- [ ] **Step 2: Copy the approved design and this plan into the new repository**

Run:

```bash
mkdir -p docs/design docs/plans
cp /Users/zzz/Documents/ChatGPT/大创项目/docs/superpowers/specs/2026-10-08-harnessscope-design.md docs/design/
cp /Users/zzz/Documents/ChatGPT/大创项目/docs/superpowers/plans/2026-10-08-harnessscope-v0.1.md docs/plans/
```

Expected: the approved product boundary travels with the implementation.

- [ ] **Step 3: Write the failing root-command test**

```go
func TestRootHelpListsStableCommands(t *testing.T) {
    var out, errOut bytes.Buffer
    code := Execute(context.Background(), []string{"--help"}, &out, &errOut)
    if code != 0 { t.Fatalf("code=%d stderr=%s", code, errOut.String()) }
    for _, name := range []string{"scan", "explain", "compare", "report", "fix", "rollback"} {
        if !strings.Contains(out.String(), name) { t.Errorf("missing %q", name) }
    }
}
```

- [ ] **Step 4: Run the test and confirm the missing package failure**

Run: `go test ./internal/cli -run TestRootHelpListsStableCommands -v`

Expected: FAIL because `Execute` and the commands do not exist.

- [ ] **Step 5: Implement the minimal root command and process entry point**

Use Cobra with `SilenceUsage: true`, `SilenceErrors: true`, injected writers, and initial command stubs that return `errors.New("command not implemented")`. `Execute` returns `0` on success, `1` for threshold errors, `2` for operational errors, and `3` for fix/rollback safety errors through typed `ExitError` values. `main.go` calls `os.Exit(cli.Execute(context.Background(), os.Args[1:], os.Stdout, os.Stderr))`.

- [ ] **Step 6: Add repository hygiene files**

`.gitignore` must contain only product-local outputs:

```gitignore
/bin/
/dist/
/coverage.out
/.hscope-test/
.DS_Store
```

Add the canonical Apache-2.0 license text with year `2026` and copyright holder `HarnessScope contributors`.

- [ ] **Step 7: Install dependency, format, and verify**

Run:

```bash
go get github.com/spf13/cobra@v1.10.1
gofmt -w cmd internal
go test ./...
go vet ./...
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add .gitignore LICENSE go.mod go.sum cmd internal docs
git commit -m "chore: bootstrap HarnessScope CLI"
```

---

### Task 2: Define the Normalized Model and Deterministic Serialization

**Files:**
- Create: `internal/model/model.go`
- Create: `internal/model/canonical.go`
- Create: `internal/model/model_test.go`
- Create: `schemas/report-v1.schema.json`

**Interfaces:**
- Produces: `model.ScanResult`, `ClientResult`, `ConfigSource`, `ConfigNode`, `Origin`, `Edge`, `Finding`, `ContextEstimate`, and enums from the design
- Produces: `model.Canonicalize(ScanResult) ScanResult`
- Produces: `model.MarshalCanonical(ScanResult) ([]byte, error)`

- [ ] **Step 1: Write failing tests for stable IDs and byte-stable JSON**

```go
func TestStableNodeID(t *testing.T) {
    got := StableNodeID("codex", "project:/repo/AGENTS.md", "instruction.root")
    if got != StableNodeID("codex", "project:/repo/AGENTS.md", "instruction.root") { t.Fatal("unstable ID") }
    if !strings.HasPrefix(got, "node_") { t.Fatalf("unexpected ID %q", got) }
}

func TestMarshalCanonicalIgnoresInputOrder(t *testing.T) {
    a := ScanResult{Analysis: Analysis{Findings: []Finding{{RuleID: "PATH-0002"}, {RuleID: "PARSE-0001"}}}}
    b := ScanResult{Analysis: Analysis{Findings: []Finding{{RuleID: "PARSE-0001"}, {RuleID: "PATH-0002"}}}}
    aj, _ := MarshalCanonical(a); bj, _ := MarshalCanonical(b)
    if !bytes.Equal(aj, bj) { t.Fatalf("non-deterministic JSON\n%s\n%s", aj, bj) }
}
```

- [ ] **Step 2: Run and confirm failure**

Run: `go test ./internal/model -v`

Expected: FAIL with undefined model symbols.

- [ ] **Step 3: Implement exact enums and structs**

Define string enums for `ClientTier`, `Scope`, `SourceFormat`, `NodeType`, `EdgeType`, `Severity`, `EvidenceStatus`, `RiskClass`, and `CompatibilityState`. `ScanResult` must separate deterministic `Analysis` from optional `RunMetadata`; JSON tags use `snake_case`. Stable IDs use SHA-256 over NUL-delimited normalized components and the first 16 lowercase hex characters.

- [ ] **Step 4: Implement canonical ordering**

Sort clients by ID, sources/nodes/edges by stable ID, findings by severity rank then rule ID then origin, and context entries by client then source. Never sort user-authored instruction text itself.

- [ ] **Step 5: Add a strict v1 JSON schema**

The schema must require `schema_version: "1.0.0"`, `analysis.clients`, `analysis.sources`, `analysis.graph`, and `analysis.findings`; set `additionalProperties: false` on stable public objects. Secret-bearing keys such as `raw_value`, `token`, `api_key`, and `authorization` must not exist in the schema.

- [ ] **Step 6: Verify and commit**

Run: `gofmt -w internal/model && go test ./internal/model -v && go test ./...`

```bash
git add internal/model schemas/report-v1.schema.json
git commit -m "feat: add normalized configuration model"
```

---

### Task 3: Build the Secret-Safe Parsing Boundary

**Files:**
- Create: `internal/secrets/redact.go`
- Create: `internal/secrets/redact_test.go`
- Create: `fixtures/security/canaries.json`

**Interfaces:**
- Produces: `secrets.Redactor`
- Produces: `RedactField(path string, value any) model.SafeValue`
- Produces: `ScrubText(input string) string`
- Consumes: `model.SafeValue` and `model.Origin`

- [ ] **Step 1: Write failing redaction-canary tests**

```go
func TestRedactorNeverReturnsRawSecrets(t *testing.T) {
    canaries := []string{"sk-test-HARNESSSCOPE-CANARY", "ghp_HARNESSSCOPE_CANARY", "Bearer HARNESSSCOPE-CANARY"}
    r := NewRedactor()
    for _, raw := range canaries {
        got := r.ScrubText("error value=" + raw)
        if strings.Contains(got, raw) || !strings.Contains(got, "[REDACTED]") { t.Fatalf("leaked %q in %q", raw, got) }
    }
}
```

Also test credential-shaped field names, PEM blocks, authorization headers, URLs with credentials, and high-entropy candidates. Include false-positive controls for ordinary paths and rule text.

- [ ] **Step 2: Run and confirm failure**

Run: `go test ./internal/secrets -v`

Expected: FAIL because the redactor does not exist.

- [ ] **Step 3: Implement early redaction**

Implement compiled, bounded regular expressions and field-name classification. `SafeValue` may retain only value kind, redacted display, secret category, and presence; raw strings must not be fields on the type. High-entropy detection requires minimum length 24 and mixed character classes to reduce path false positives.

- [ ] **Step 4: Add an object walker for parsed maps**

Add `RedactTree(value any, fieldPath string) any`, returning a deep copy whose scalar secrets are replaced by `SafeValue`. Do not mutate parser-owned structures and do not serialize raw parse errors without `ScrubText`.

- [ ] **Step 5: Verify and commit**

Run: `gofmt -w internal/secrets && go test ./internal/secrets -v && go test ./...`

```bash
git add internal/secrets fixtures/security
git commit -m "feat: enforce secret-safe parsed values"
```

---

### Task 4: Implement Bounded Environment and Source Discovery

**Files:**
- Create: `internal/discovery/environment.go`
- Create: `internal/discovery/sources.go`
- Create: `internal/discovery/discovery_test.go`

**Interfaces:**
- Produces: `discovery.Environment{GOOS, HomeDir, AppDataDir, PathEntries}`
- Produces: `FindExecutable(name string) DetectionResult`
- Produces: `WalkAncestors(cwd string) ([]string, error)`
- Produces: `InspectKnownPath(logical string) model.ConfigSource`

- [ ] **Step 1: Write failing isolated-home and symlink-cycle tests**

Use `t.TempDir()` for both home and workspace. Assert ancestor order is workspace-to-root, known missing paths are represented with `Exists=false`, logical and canonical symlink paths are both retained, and a cycle returns a `SOURCE` limitation instead of recursively walking.

- [ ] **Step 2: Run and confirm failure**

Run: `go test ./internal/discovery -v`

Expected: FAIL with undefined discovery functions.

- [ ] **Step 3: Implement environment injection and app-data paths**

`NewEnvironment(goos, home string, path []string)` must be usable in tests. Production defaults are:

```text
darwin: ~/Library/Application Support/HarnessScope
linux:  ${XDG_DATA_HOME:-~/.local/share}/harnessscope
```

Never read the real home in tests; callers pass an `Environment`.

- [ ] **Step 4: Implement bounded known-path inspection**

Use `os.Lstat`, `filepath.EvalSymlinks`, a visited-inode/path set, and explicit adapter-provided candidate paths. Do not expose a general recursive home crawler.

- [ ] **Step 5: Verify and commit**

Run: `gofmt -w internal/discovery && go test ./internal/discovery -race -v && go test ./...`

```bash
git add internal/discovery
git commit -m "feat: add bounded configuration discovery"
```

---

### Task 5: Define the Adapter Contract and Registry

**Files:**
- Create: `internal/adapters/adapter.go`
- Create: `internal/adapters/registry.go`
- Create: `internal/adapters/registry_test.go`

**Interfaces:**
- Produces: `adapters.ClientAdapter` exactly matching the approved design
- Produces: `Registry.Register(adapter ClientAdapter) error`
- Produces: `Registry.Select(names []string) ([]ClientAdapter, error)`

- [ ] **Step 1: Write the failing registry tests**

```go
func TestRegistryRejectsDuplicateClientID(t *testing.T) {
    r := NewRegistry()
    if err := r.Register(fakeAdapter{id: "codex"}); err != nil { t.Fatal(err) }
    if err := r.Register(fakeAdapter{id: "codex"}); err == nil { t.Fatal("expected duplicate error") }
}

func TestSelectAllUsesStableOrder(t *testing.T) {
    r := NewRegistry(fakeAdapter{id:"opencode"}, fakeAdapter{id:"codex"})
    got, err := r.Select([]string{"all"})
    if err != nil || got[0].ID() != "codex" { t.Fatalf("got=%v err=%v", got, err) }
}
```

- [ ] **Step 2: Run and confirm failure**

Run: `go test ./internal/adapters -v`

Expected: FAIL with undefined registry.

- [ ] **Step 3: Implement the interface and capability metadata**

Add `ID() string`, then the design methods `Detect`, `DiscoverSources`, `Parse`, `Resolve`, `Capabilities`, and `Compatibility`. Parse results must contain normalized safe nodes, adapter limitations, and redacted errors; no raw parsed map crosses the adapter boundary.

- [ ] **Step 4: Verify and commit**

Run: `gofmt -w internal/adapters && go test ./internal/adapters -v && go test ./...`

```bash
git add internal/adapters
git commit -m "feat: define client adapter contract"
```

---

### Task 6: Implement the Verified Codex Adapter

**Files:**
- Create: `internal/adapters/codex/adapter.go`
- Create: `internal/adapters/codex/parse.go`
- Create: `internal/adapters/codex/resolve.go`
- Create: `internal/adapters/codex/adapter_test.go`
- Create: `fixtures/codex/basic/`
- Create: `fixtures/codex/nested/`
- Create: `fixtures/codex/malformed/`
- Create: `fixtures/codex/version-drift/`
- Create: `docs/evidence/codex-0.162.0-alpha.2.md`

**Interfaces:**
- Produces: `codex.New(env discovery.Environment, redactor secrets.Redactor) adapters.ClientAdapter`
- Produces normalized instruction, skill, MCP, and source nodes with provenance

- [ ] **Step 1: Create synthetic Codex fixtures and failing golden tests**

Fixtures must cover a fake home `.codex/config.toml`, user `AGENTS.md`, repository `AGENTS.md`, nested `AGENTS.md`, one valid local MCP command, one missing command, a skill with `SKILL.md`, malformed TOML, and an installed version outside the verified range. Golden assertions name exact effective sources and `OVERRIDES`/`LOADS` edges.

- [ ] **Step 2: Run and confirm failure**

Run: `go test ./internal/adapters/codex -v`

Expected: FAIL because the adapter is absent.

- [ ] **Step 3: Implement detection and declared-path discovery**

Detect `codex` through the injected PATH, invoke `codex --version` with a two-second context timeout, and scrub stderr. Discover only adapter-declared global files plus ancestor/nested instruction files relevant to the supplied cwd. Record missing candidates without scanning unrelated home content.

The first verified metadata entry is exact version `0.162.0-alpha.2`, the locally inspected version on 2026-10-08. Record retrieval date, supported fields, fixture names, and these primary references in `docs/evidence/codex-0.162.0-alpha.2.md`:

```text
https://learn.chatgpt.com/docs/config-file/config-basic
https://learn.chatgpt.com/docs/agent-configuration/agents-md
```

Any other version remains `COMPATIBILITY_UNKNOWN` until a later evidence commit expands the range.

- [ ] **Step 4: Implement TOML and Markdown parsing**

Add `github.com/pelletier/go-toml/v2@v2.2.4` and `gopkg.in/yaml.v3@v3.0.1`. Parse through local raw structs, immediately convert literals with `RedactTree`, and emit normalized nodes with field paths such as `mcp_servers.playwright.command`. Markdown instructions retain text only as a redacted safe payload plus byte/code-point counts.

- [ ] **Step 5: Implement Codex precedence resolution**

Encode precedence in named rule constants with evidence references and verified-version metadata. For unsupported versions, preserve syntax/source findings but set precedence conclusions to `COMPATIBILITY_UNKNOWN` and suppress `CONFIRMED` override claims.

- [ ] **Step 6: Verify and commit**

Run: `gofmt -w internal/adapters/codex && go test ./internal/adapters/codex -race -v && go test ./...`

```bash
git add internal/adapters/codex fixtures/codex go.mod go.sum
git commit -m "feat: add verified Codex adapter"
```

---

### Task 7: Implement the Verified Claude Code Adapter

**Files:**
- Create: `internal/adapters/claude/adapter.go`
- Create: `internal/adapters/claude/parse.go`
- Create: `internal/adapters/claude/resolve.go`
- Create: `internal/adapters/claude/adapter_test.go`
- Create: `fixtures/claude/basic/`
- Create: `fixtures/claude/imports/`
- Create: `fixtures/claude/malformed/`
- Create: `fixtures/claude/version-drift/`
- Create: `docs/evidence/claude-2.1.259.md`

**Interfaces:**
- Produces: `claude.New(env discovery.Environment, redactor secrets.Redactor) adapters.ClientAdapter`
- Produces normalized instruction, import, hook, MCP, and source nodes with provenance

- [ ] **Step 1: Create synthetic Claude fixtures and failing golden tests**

Cover fake global/project/local settings, root and imported `CLAUDE.md`, a direct import cycle, hooks with executable and missing scripts, duplicate MCP names, malformed JSON, and an out-of-range client version. Assert import-cycle findings are `CONFIRMED` without aborting unrelated analysis.

- [ ] **Step 2: Run and confirm failure**

Run: `go test ./internal/adapters/claude -v`

Expected: FAIL because the adapter is absent.

- [ ] **Step 3: Implement detection, source discovery, and parsing**

Detect `claude` with the same bounded process helper. Parse JSON settings through typed raw structs and Markdown imports with an explicit visited set. Convert hooks and MCP entries to normalized nodes only after redaction.

The first verified metadata entry is exact version `2.1.259`, the locally inspected version on 2026-10-08. Record retrieval date, version-dependent exclusions, fixture names, and these primary references in `docs/evidence/claude-2.1.259.md`:

```text
https://code.claude.com/docs/en/memory
https://code.claude.com/docs/en/settings
https://code.claude.com/docs/en/mcp
```

The evidence note must explicitly exclude newer behavior such as direct `AGENTS.md` loading introduced after this version. Any other version remains `COMPATIBILITY_UNKNOWN` until validated fixtures expand the range.

- [ ] **Step 4: Implement Claude precedence and import resolution**

Keep global, project, and local scopes distinct. Add `IMPORTS`, `OVERRIDES`, and `EFFECTIVE_AS` edges with rule IDs. Downgrade version-dependent conclusions when compatibility is unknown.

- [ ] **Step 5: Verify and commit**

Run: `gofmt -w internal/adapters/claude && go test ./internal/adapters/claude -race -v && go test ./...`

```bash
git add internal/adapters/claude fixtures/claude
git commit -m "feat: add verified Claude Code adapter"
```

---

### Task 8: Assemble the Provenance Graph and Deterministic Analyzers

**Files:**
- Create: `internal/resolver/graph.go`
- Create: `internal/resolver/graph_test.go`
- Create: `internal/analyzers/analyzers.go`
- Create: `internal/analyzers/structural.go`
- Create: `internal/analyzers/context.go`
- Create: `internal/analyzers/analyzers_test.go`

**Interfaces:**
- Produces: `resolver.Build(clientResults []model.ClientResult) model.Graph`
- Produces: `analyzers.Run(ctx context.Context, input model.Analysis) []model.Finding`
- Produces: `analyzers.EstimateContext(nodes []model.ConfigNode) []model.ContextEstimate`

- [ ] **Step 1: Write failing graph invariant and analyzer table tests**

Test stable graph output under shuffled inputs, cycle representation without recursion failure, dangling-reference rejection, duplicate MCP detection, missing executable/path findings, explicit instruction contradiction patterns, and `UNKNOWN` propagation from compatibility limitations.

- [ ] **Step 2: Run and confirm failure**

Run: `go test ./internal/resolver ./internal/analyzers -v`

Expected: FAIL with missing packages.

- [ ] **Step 3: Implement graph assembly and validation**

Deduplicate nodes by stable ID, retain all distinct origins, sort adjacency lists, and expose `Validate() error`. Cycles are graph facts and findings, not fatal build errors.

- [ ] **Step 4: Implement the v0.1 rule registry**

Use fixed IDs including `PARSE-0001`, `SOURCE-0001`, `OVERRIDE-0001`, `MCP-0001`, `PATH-0001`, `ENV-0001`, `RULE-0001`, `HOOK-0001`, `CONTEXT-0001`, and `PORTABLE-0001`. Each constructor must require reason, impact, remediation, evidence, origins, and affected clients so incomplete findings cannot compile.

- [ ] **Step 5: Implement conservative context estimates**

Always report bytes and Unicode code points. Without an authoritative known tokenizer, report `min_tokens = ceil(codepoints/6)` and `max_tokens = ceil(codepoints/2)` with method `ESTIMATED_RANGE`; never label the range exact. Separate always-loaded, conditional, MCP schema, lazy skill, duplicate, and unknown categories.

- [ ] **Step 6: Verify and commit**

Run: `gofmt -w internal/resolver internal/analyzers && go test ./internal/resolver ./internal/analyzers -race -v && go test ./...`

```bash
git add internal/resolver internal/analyzers
git commit -m "feat: resolve provenance graph and findings"
```

---

### Task 9: Deliver `scan`, `explain`, and `compare`

**Files:**
- Create: `internal/cli/runtime.go`
- Create: `internal/cli/scan.go`
- Create: `internal/cli/explain.go`
- Create: `internal/cli/compare.go`
- Create: `internal/cli/commands_test.go`
- Create: `internal/report/terminal.go`
- Create: `internal/report/terminal_test.go`

**Interfaces:**
- Produces: `cli.Runtime.Scan(ctx, ScanOptions) (model.ScanResult, error)`
- Produces: `report.WriteTerminal(io.Writer, model.ScanResult) error`
- Consumes: adapter registry, resolver, analyzers, and injected environment

- [ ] **Step 1: Write failing command-level tests**

Invoke `Execute` with injected fake adapters and temp app-data. Assert `scan . --client all --no-report` is read-only, `--fail-on high` returns code `1`, `explain mcp.playwright --client codex` prints the full origin chain, and `compare codex claude` reports divergent normalized elements.

- [ ] **Step 2: Run and confirm failure**

Run: `go test ./internal/cli ./internal/report -v`

Expected: FAIL because command handlers still return the initial not-implemented error.

- [ ] **Step 3: Implement scan orchestration and terminal output**

Run adapters independently so one parse failure does not cancel other clients. Print detected clients, compatibility tier, severity/evidence counts, and findings with location, reason, impact, and remediation. Missing clients are skipped with an informational limitation.

- [ ] **Step 4: Implement explain and compare**

`explain` resolves stable or human aliases and prints sources considered, load conditions, override chain, effective node, and uncertainty. `compare` aligns nodes by normalized type/name and reports `same`, `missing`, or `different` without claiming semantic equivalence beyond normalized fields.

- [ ] **Step 5: Verify exit contracts and commit**

Run: `gofmt -w internal/cli internal/report && go test ./internal/cli ./internal/report -race -v && go test ./...`

```bash
git add internal/cli internal/report/terminal.go internal/report/terminal_test.go
git commit -m "feat: add scan explain and compare commands"
```

---

### Task 10: Generate Stable JSON and a Self-Contained Offline HTML Report

**Files:**
- Create: `internal/report/json.go`
- Create: `internal/report/json_test.go`
- Create: `internal/report/html.go`
- Create: `internal/report/html_test.go`
- Create: `internal/report/assets/report.css`
- Create: `internal/report/assets/report.js`
- Create: `internal/cli/report.go`
- Create: `web/package.json`
- Create: `web/package-lock.json`
- Create: `web/tsconfig.json`
- Create: `web/src/report.ts`
- Create: `web/src/report.css`
- Create: `web/tests/report.spec.ts`

**Interfaces:**
- Produces: `report.WriteJSON(io.Writer, model.ScanResult) error`
- Produces: `report.WriteHTML(io.Writer, model.ScanResult) error`
- Produces: `report.ReadJSON(io.Reader) (model.ScanResult, error)`

- [ ] **Step 1: Write failing serialization and offline-report tests**

Assert canonical JSON is byte-identical for shuffled inputs; canary secrets are absent; HTML contains no `http://`, `https://`, remote fonts, or external script/style tags; malicious source names are escaped; and a no-JavaScript summary/table is present.

- [ ] **Step 2: Run and confirm failure**

Run: `go test ./internal/report -run 'Test(JSON|HTML)' -v`

Expected: FAIL with missing report functions.

- [ ] **Step 3: Implement JSON codec and schema-version checks**

Call `model.Canonicalize` before encoding. Reject unknown major schema versions in `ReadJSON`; preserve unknown minor fields only through a clearly separated extensions map. Run every returned error through the redactor before CLI display.

- [ ] **Step 4: Build the typed frontend assets**

Install exact development dependencies with `npm install --save-dev --save-exact typescript@5.9.3 esbuild@0.25.11 @playwright/test@1.56.1`, committing `web/package-lock.json`. `npm run build` must compile `web/src/report.ts` to `internal/report/assets/report.js`, minify both frontend assets, disable source maps, and avoid runtime packages. `npm ci && npm run build && git diff --exit-code internal/report/assets` is the reproducibility check.

- [ ] **Step 5: Implement one-file HTML rendering**

Use `//go:embed assets/*`, `html/template`, escaped JSON embedded as `application/json`, inline CSS/JS, SVG-only local diagrams, keyboard-focusable filters, text severity labels, and a static summary/findings table outside script-dependent elements.

- [ ] **Step 6: Add browser and visual regression tests**

Use Playwright Chromium against a generated synthetic report. Assert keyboard navigation, color-independent severity labels, filtering, no network requests, and readable content with JavaScript disabled. Capture one reviewed Linux snapshot at viewport `1440x1000`; run visual comparison only on Ubuntu CI to avoid cross-platform font noise.

- [ ] **Step 7: Wire report paths and `--open`**

Default output is `<app-data>/reports/<analysis-id>/report.json` and `report.html`. `--open` uses `open` on macOS and `xdg-open` on Linux only after successful generation; command execution is argument-array based, never shell interpolation. `--no-report` suppresses both files.

- [ ] **Step 8: Implement saved-report rendering**

`hscope report --from report.json --html report.html` must call `ReadJSON` then `WriteHTML` without discovering or reading live client configuration.

- [ ] **Step 9: Verify and commit**

Run: `cd web && npm ci && npm run build && npx playwright install chromium && npm run test:visual && cd .. && gofmt -w internal/report internal/cli && go test ./internal/report ./internal/cli -race -v && go test ./...`

```bash
git add internal/report internal/cli schemas/report-v1.schema.json web
git commit -m "feat: add offline JSON and HTML reports"
```

---

### Task 11: Implement Conservative Fix Planning, Transactions, and Rollback

**Files:**
- Create: `internal/fixes/plan.go`
- Create: `internal/fixes/transaction.go`
- Create: `internal/fixes/transaction_test.go`
- Create: `internal/cli/fix.go`
- Create: `internal/cli/rollback.go`
- Create: `schemas/fix-plan-v1.schema.json`

**Interfaces:**
- Produces: `fixes.Plan(result model.ScanResult, selected []string) (model.FixPlan, error)`
- Produces: `fixes.Apply(ctx context.Context, plan model.FixPlan) (TransactionResult, error)`
- Produces: `fixes.Rollback(ctx context.Context, backupID string) error`

- [ ] **Step 1: Write failing full round-trip tests**

For each initial `SAFE` operation, test `scan -> plan -> apply -> rescan -> rollback -> rescan`. Assert source-hash mismatch refusal, `0600` backup/manifest permissions, unrelated-byte preservation, atomic replacement, automatic rollback after a failed postcondition, and absence of canary secrets in manifests/errors.

- [ ] **Step 2: Run and confirm failure**

Run: `go test ./internal/fixes -v`

Expected: FAIL because fix planning is absent.

- [ ] **Step 3: Implement narrow fix-plan generation**

Generate plans only for the three approved `SAFE` operations: same-source/same-scope byte-identical duplicate removal, executable-bit repair for an existing local shebang hook, and canonical path normalization when both paths resolve to the same object. Classify every other proposed mutation as `REVIEW` or `MANUAL`.

- [ ] **Step 4: Implement transaction protocol**

Rehash and reparse targets; create user-only backup files and redacted manifest; write replacements beside originals; `Sync`, preserve permissions, and `Rename`; rescan postconditions; restore every touched file on any failure. Backup IDs are timestamp plus random 8-byte hex, never user path content.

- [ ] **Step 5: Wire CLI safety behavior**

`hscope fix` and `hscope fix --dry-run` print the plan only. `--apply` applies only `SAFE` fixes unless a named `REVIEW` fix ID and interactive confirmation are both present. Non-TTY execution refuses interactive review fixes. `rollback` restores only the selected manifest.

- [ ] **Step 6: Verify and commit**

Run: `gofmt -w internal/fixes internal/cli && go test ./internal/fixes ./internal/cli -race -v && go test ./...`

```bash
git add internal/fixes internal/cli schemas/fix-plan-v1.schema.json
git commit -m "feat: add reversible configuration fixes"
```

---

### Task 12: Add Cursor and OpenCode Preview Adapters

**Files:**
- Create: `internal/adapters/cursor/adapter.go`
- Create: `internal/adapters/cursor/adapter_test.go`
- Create: `internal/adapters/opencode/adapter.go`
- Create: `internal/adapters/opencode/adapter_test.go`
- Create: `internal/formats/jsonc.go`
- Create: `internal/formats/jsonc_test.go`
- Create: `fixtures/cursor/preview/`
- Create: `fixtures/opencode/preview/`

**Interfaces:**
- Produces: `cursor.New(...) adapters.ClientAdapter`
- Produces: `opencode.New(...) adapters.ClientAdapter`
- Guarantees: tier is always `PREVIEW`; unsupported precedence evidence is always `UNKNOWN`

- [ ] **Step 1: Write failing preview-boundary tests**

Test source discovery, syntax failures, MCP/path findings, and explicit absence of `CONFIRMED` `OVERRIDES`, `SHADOWS`, or `EFFECTIVE_AS` claims. Verify terminal, JSON, and HTML all display `PREVIEW`.

- [ ] **Step 2: Run and confirm failure**

Run: `go test ./internal/adapters/cursor ./internal/adapters/opencode -v`

Expected: FAIL because preview adapters are absent.

- [ ] **Step 3: Implement Cursor preview inspection**

First implement a bounded JSONC lexer that removes line/block comments and trailing commas only outside quoted strings; its tests must preserve comment-like text inside strings and reject unterminated comments. Then inspect only documented adapter-declared project/user sources represented in fixtures, parse supported JSON/JSONC and rule files, and emit syntax/source/MCP/path nodes. Keep precedence capability false.

- [ ] **Step 4: Implement OpenCode preview inspection**

Inspect only declared JSON/JSONC configuration and instruction sources, parse supported MCP/path structures, and emit preview limitations. Keep precedence capability false.

- [ ] **Step 5: Verify and commit**

Run: `gofmt -w internal/adapters/cursor internal/adapters/opencode internal/formats && go test ./internal/adapters/cursor ./internal/adapters/opencode ./internal/formats -race -v && go test ./...`

```bash
git add internal/adapters/cursor internal/adapters/opencode internal/formats fixtures/cursor fixtures/opencode
git commit -m "feat: add Cursor and OpenCode preview adapters"
```

---

### Task 13: Harden with Fuzzing, Public Demo, Documentation, and Release CI

**Files:**
- Create: `internal/secrets/fuzz_test.go`
- Create: `internal/resolver/fuzz_test.go`
- Create: `internal/report/fuzz_test.go`
- Create: `demo/conflicted-workspace/`
- Create: `demo/run.sh`
- Create: `docs/architecture.md`
- Create: `README.md`
- Create: `README.zh-CN.md`
- Create: `SECURITY.md`
- Create: `CONTRIBUTING.md`
- Create: `.github/workflows/ci.yml`
- Create: `.github/workflows/release.yml`
- Create: `scripts/smoke-release.sh`

**Interfaces:**
- Produces: reproducible contributor workflow and release archives for `darwin/linux` × `amd64/arm64`
- Consumes: complete CLI and committed synthetic fixtures

- [ ] **Step 1: Add fuzz/property tests for safety invariants**

Seed fuzzers with valid, malformed, cyclic, secret-bearing, and hostile HTML inputs. Properties: no panic, no raw canary propagation, valid graph references after canonicalization, deterministic JSON for equivalent orderings, and valid UTF-8 output.

- [ ] **Step 2: Build the conflicted demo workspace**

Create only synthetic data: global/project/nested instruction overlap, duplicate MCP names, a missing command, a portable-path issue, and one literal canary that must render as `[REDACTED]`. Add `demo/conflicted-workspace/EXPECTED.md` listing the exact expected findings. `demo/run.sh` must build `hscope`, scan the fixture, print report paths, and finish without interaction; keep the terminal sequence short enough to record as a 15-second terminal-to-graph demonstration.

- [ ] **Step 3: Write English and Chinese documentation**

Both READMEs must include installation from checksummed release archives, five-minute `hscope scan . --open`, supported tiers, privacy guarantees, sample terminal output, limitations, and roadmap. Limit public behavior claims to committed fixtures and release-gate evidence. Include a report screenshot or GIF only when the referenced asset is committed. `SECURITY.md` provides private disclosure instructions without inventing an email address; use GitHub private vulnerability reporting. `CONTRIBUTING.md` forbids real credentials and real user configurations in fixtures.

- [ ] **Step 4: Add CI**

On macOS and Linux run:

```bash
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/hscope
```

Also run `npm ci`, rebuild committed assets, execute Playwright functional tests everywhere and visual comparison on Ubuntu, scan repository text for fixture canaries outside their allowed synthetic input files, and run the demo assertions.

- [ ] **Step 5: Audit dependency licensing and attribution**

Generate `THIRD_PARTY_NOTICES.md` from the exact locked Go and npm dependency graph. Verify every shipped dependency is compatible with Apache-2.0 distribution, record its module/package, version, license, and upstream URL, and fail CI when lockfiles change without a corresponding notice update. Do not copy implementation code from competing projects.

- [ ] **Step 6: Add release builds and clean-directory smoke tests**

For tags `v*`, build with `CGO_ENABLED=0`, archive each OS/architecture binary with license and READMEs, emit SHA-256 checksums, unpack each compatible artifact into a new temporary directory, and run `hscope --help` plus a synthetic `hscope scan ... --no-report`. Do not publish until all matrix jobs pass.

- [ ] **Step 7: Run the release gate locally**

Run:

```bash
gofmt -w cmd internal
go test ./...
go test -race ./...
go vet ./...
cd web && npm ci && npm run build && cd ..
go build -trimpath -o bin/hscope ./cmd/hscope
./bin/hscope --help
./scripts/smoke-release.sh ./bin/hscope
git diff --check
```

Expected: every command passes, reports remain offline, and no canary appears in outputs.

- [ ] **Step 8: Commit**

```bash
git add internal demo docs README.md README.zh-CN.md SECURITY.md CONTRIBUTING.md THIRD_PARTY_NOTICES.md .github scripts web
git commit -m "docs: prepare HarnessScope v0.1 release"
```

---

## Final Release Gate

- [ ] Run `go test ./...`, `go test -race ./...`, `go vet ./...`, and the clean-directory smoke script from a fresh checkout.
- [ ] Confirm `git status --short` is empty and `git diff --check` passes.
- [ ] Confirm all ten acceptance criteria in the approved design have direct test or documentation evidence.
- [ ] Inspect generated terminal, JSON, and HTML output for the public demo and verify no real machine paths or secrets are present.
- [ ] Build all four release targets and verify their SHA-256 checksum file.
- [ ] Create the GitHub repository and push only after the local release gate passes and the user confirms the final repository name/account.
