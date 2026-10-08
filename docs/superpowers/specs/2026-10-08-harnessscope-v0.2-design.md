# HarnessScope v0.2 Local Control Center Design

Date: 2026-10-08  
Status: approved design  
Target branch: `codex/harnessscope-v0.2`

## 1. Goal

HarnessScope v0.2 turns the v0.1 offline inspector into a visually distinctive local control center without abandoning its privacy and distribution advantages. The release must improve both public-demo appeal and technical depth:

- a polished interactive dashboard that is easy to understand in a short recording;
- stronger, evidence-aware diagnostics and configuration-drift analysis;
- safe browser-driven fix and rollback workflows;
- a shareable diagnostic bundle that contains no raw configuration or machine-specific paths;
- continued delivery as one offline Go binary with no telemetry and no cloud dependency.

The primary entry point is:

```text
hscope serve [path] --open
```

The existing CLI and self-contained report remain supported.

## 2. Non-goals

v0.2 will not:

- become an Electron, Tauri, or hosted SaaS application;
- listen on a non-loopback interface;
- fetch remote OpenCode configuration or contact MCP servers;
- execute hooks, MCP commands, or agent clients except their existing bounded `--version` probes;
- introduce semantic or LLM-based similarity scoring;
- claim verified precedence for preview adapters;
- silently apply a fix from a browser action;
- include raw configuration, backup contents, environment values, or user-identifying paths in an exported bundle.

## 3. User experience

### 3.1 Startup

`hscope serve [path]` fixes the scan root for the lifetime of the process, chooses an available loopback port by default, performs an initial scan, and prints the authenticated local URL once to the interactive terminal. `--open` launches the default browser only after the listener and initial scan are ready. Structured logs and error messages never repeat the token-bearing URL.

The browser URL carries a random session token in the URL fragment. The fragment is never sent to the HTTP server. Frontend boot code reads it into memory, removes it from the visible URL with `history.replaceState`, and sends it in the `X-HarnessScope-Token` header for every API request.

### 3.2 Dashboard layout

The interface uses a dark technical visual system without external fonts or assets:

- fixed left rail for Overview, Graph, Findings, Compare, Fix Center, Drift, and Export;
- compact top bar with scan root, revision, client tiers, rescan control, and connection state;
- overview risk cards for client coverage, findings, estimated context, and portability;
- central interactive provenance graph with pan, zoom, search, scope/client filters, evidence styling, and keyboard selection;
- right-hand inspector showing the selected node, redacted attributes, origin chain, load condition, and evidence boundary;
- findings table with severity/evidence filters and direct graph focus;
- comparison matrix showing which normalized elements occur in each client;
- fix center with redacted patches, risk labels, preconditions, backup IDs, and rollback history;
- drift view comparing the current sanitized snapshot against a user-selected local baseline.

The visual style uses deep navy surfaces, subtle grid texture, cyan/purple accents, restrained glow, crisp monospace labels, and animated transitions that respect `prefers-reduced-motion`. Severity is always represented by text and shape in addition to color. Desktop widths are the primary demo target, while all workflows remain usable at 768 px.

### 3.3 Mutation flow

Browser fixes preserve v0.1 safety semantics:

1. The user requests a fresh plan.
2. The server returns only redacted patches and a scan revision.
3. The user selects named `SAFE` fixes and sees a confirmation summary.
4. Apply sends the selected IDs and expected revision.
5. The server rejects stale revisions, rescans, replans, verifies source hashes, creates backups, applies, and rescans postconditions.
6. Success returns backup IDs and the new revision; failure automatically rolls back and returns a redacted error.

`REVIEW` and `MANUAL` plans are display-only in v0.2. The control center never provides a browser bypass for them.

## 4. Architecture

```text
CLI command
   |
   v
local server (loopback + session guard)
   |-------------------------|
   v                         v
scan service             transaction service
   |                         |
   v                         v
immutable safe state     existing fix/rollback core
   |
   |----> JSON API ----> embedded TypeScript/CSS dashboard
   |----> snapshot store
   `----> sanitized ZIP exporter
```

### 4.1 New packages

- `internal/server`: listener lifecycle, authentication/origin middleware, API routes, revisioned state, and browser-open coordination.
- `internal/snapshots`: sanitized snapshot persistence, listing, loading, and normalized drift computation.
- `internal/export`: deterministic ZIP diagnostic bundles and manifest hashes.
- `web/dashboard`: TypeScript/CSS source and Playwright tests for the control center.

The existing `Runtime.Scan`, resolver, analyzers, fix planner, transaction engine, and report writers remain the authoritative domain logic. HTTP handlers orchestrate these services; they do not duplicate parsing or safety rules.

### 4.2 Revisioned state

The server stores one immutable `DashboardState` under a read/write lock:

```text
schema_version
revision
scanned_at
workspace_display_path
scan_result
fix_plans
backup_history
drift_summary (optional)
```

Revision is a monotonic integer scoped to the server process. Every successful rescan, fix, or rollback publishes a new state atomically. Clients send the revision for mutations; stale requests receive HTTP 409 and no filesystem change.

### 4.3 API

All `/api/v1/*` routes require the session header, reject cross-origin requests, return `Cache-Control: no-store`, and use JSON unless noted.

| Method | Route | Purpose |
|---|---|---|
| GET | `/api/v1/state` | Current immutable dashboard state |
| POST | `/api/v1/rescan` | Produce and publish a new scan revision |
| GET | `/api/v1/explain?id=...` | Exact node and origin-chain detail |
| GET | `/api/v1/compare?left=...&right=...` | Normalized comparison matrix |
| POST | `/api/v1/fixes/plan` | Fresh redacted fix plans |
| POST | `/api/v1/fixes/apply` | Apply named SAFE plans for an expected revision |
| GET | `/api/v1/backups` | Redacted backup history |
| POST | `/api/v1/rollback` | Restore one selected backup ID |
| GET | `/api/v1/snapshots` | List local sanitized baselines |
| POST | `/api/v1/snapshots` | Save the current sanitized state as a baseline |
| POST | `/api/v1/drift` | Compare current state with a named baseline |
| POST | `/api/v1/export` | Stream a sanitized diagnostic ZIP |

Request bodies are size-limited. Unknown fields are rejected. IDs and names use strict allowlists. The UI never sends arbitrary filesystem paths after startup.

## 5. Diagnostic upgrades

The following shared analyzers are added without changing preview-tier guarantees:

- `CLIENT-0001`: installed version is outside the adapter's verified version set;
- `CMD-0001`: a non-absolute MCP or hook command cannot be resolved from the captured executable search path;
- `PATH-0002`: an existing absolute command or working-directory path is machine-specific and non-portable;
- `CONFIG-0001`: the same normalized setting or MCP name has divergent display-safe values across clients or sources;
- `SECRET-0001`: a credential-shaped field is present in committed/project configuration; only category and origin are reported;
- `SOURCE-0003`: a discovered symlink resolves outside the fixed scan root or declared user configuration root;
- `DRIFT-0001`: a normalized client, source, node, finding, or compatibility state differs from the selected baseline.

Confirmed facts such as local file existence may use `CONFIRMED`. Behavioral conclusions inherit the weakest relevant adapter evidence. Preview adapters therefore continue to yield `UNKNOWN` for effective-behavior conclusions.

## 6. Snapshot and drift model

Snapshots are canonical, sanitized report documents stored under the HarnessScope application-data directory. A snapshot name must match `[A-Za-z0-9][A-Za-z0-9._-]{0,63}`. Files use `0600`; directories use `0700`.

Drift compares stable IDs and selected normalized fields. It emits added, removed, and changed entries without exposing raw values. Ordering is deterministic. Schema-major mismatches are rejected with an actionable error; no implicit migration is attempted in v0.2.

## 7. Diagnostic bundle

The export endpoint creates a deterministic ZIP containing:

```text
manifest.json
report.json
report.html
drift.json                 (when a baseline comparison is active)
README.txt
```

`manifest.json` includes HarnessScope version, schema versions, client compatibility tiers, generation time, and SHA-256 hashes for bundle members. It excludes hostnames, usernames, absolute paths, environment values, raw configurations, and backup contents. The exporter rescans its generated members for known canaries and credential patterns before returning the archive.

## 8. Security model

- Listen on `127.0.0.1` or `::1` only; reject attempts to configure another host.
- Generate at least 32 random bytes for each server session.
- Require the session header on every API request, including reads.
- Reject missing or foreign `Origin` headers on mutation requests; do not enable CORS.
- Validate `Host` as a loopback literal with the active port to reduce DNS-rebinding risk.
- Apply request and response size limits and explicit content types.
- Set CSP, `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer`, and `Cache-Control: no-store`.
- Escape all text through DOM text nodes; do not use `innerHTML` for report values.
- Keep the session token in memory after the one-time interactive launch URL; never place it in structured logs, errors, persisted state, or exports.
- Redact errors before terminal or API output.
- Shut down cleanly on context cancellation without abandoning an active transaction.

## 9. Error handling

API errors use a stable envelope with `code`, `message`, and optional safe details. Expected mappings are:

- 400 for malformed or invalid requests;
- 401 for a missing or incorrect session token;
- 403 for invalid origin or host;
- 404 for unknown normalized IDs, snapshots, or backups;
- 409 for stale revisions or concurrent source changes;
- 422 for a request that cannot satisfy fix safety rules;
- 500 for redacted operational failures.

The dashboard preserves the last valid state when rescan fails, shows a non-blocking error panel, and never fabricates a successful revision.

## 10. Compatibility

- Existing v1 report JSON remains readable and writable.
- Existing CLI commands and exit codes remain unchanged.
- `serve`, `snapshot`, and `export` are additive commands.
- The static `report.html` remains self-contained and usable without the server.
- v0.1 backup manifests remain valid for rollback.
- No confirmed Cursor or OpenCode precedence edges are added solely for the dashboard.

## 11. Testing strategy

### 11.1 Go tests

- `httptest` coverage for every route, auth failure, origin/host rejection, body limits, stale revisions, cancellation, and redacted errors;
- race tests for concurrent state reads and rescans;
- snapshot round trips, permissions, schema rejection, and deterministic drift ordering;
- deterministic ZIP bytes under an injected clock, stable member ordering, manifest-hash verification, and canary leakage checks;
- analyzer fixtures for every new rule with confirmed/unknown evidence boundaries;
- full fix apply, rescan, rollback, and stale-revision refusal through the HTTP layer.

### 11.2 Browser tests

Playwright verifies navigation, graph selection, filters, compare, fix confirmation, rollback, drift, export, keyboard access, reduced motion, no external requests, and useful behavior at 1440x1000 and 768x900. A Linux visual snapshot covers the overview and graph. Tests use only synthetic server state.

### 11.3 Release gates

CI runs Go unit/race/vet tests, TypeScript checking, asset reproducibility, Playwright, the synthetic dashboard demo, secret-canary scans, four cross-builds, archive checksums, and a clean-directory smoke test.

## 12. Acceptance criteria

v0.2 is complete only when all conditions hold:

1. `hscope serve <fixture> --open` reaches a ready dashboard without an external network request.
2. The dashboard provides usable Overview, Graph, Findings, Compare, Fix Center, Drift, and Export views.
3. Graph pan/zoom, filtering, keyboard selection, and origin inspection work on the committed demo fixture.
4. Every API route rejects a missing token; mutations additionally reject invalid origin, host, revision, and risk class.
5. A SAFE duplicate fix completes plan, confirmation, backup, apply, rescan, and rollback from the browser test.
6. Every new diagnostic rule has a synthetic positive case, negative case, and evidence-level assertion.
7. A saved baseline produces deterministic added/removed/changed drift output after a fixture mutation.
8. An exported ZIP contains only the declared members, passes its manifest hashes, and contains neither fixture canaries nor real machine paths.
9. Existing CLI, report schema v1, static HTML, fix, and rollback tests remain green.
10. The dashboard is accessible without color-only severity cues and respects reduced motion.
11. macOS/Linux amd64/arm64 release builds and SHA-256 verification pass.
12. README assets and demo claims are generated from committed synthetic fixtures and verified release-gate output.

## 13. Delivery sequence

1. Add server security, immutable state, and the `serve` command.
2. Add the dashboard shell and read-only Overview/Graph/Findings/Compare flows.
3. Add diagnostics, snapshots, and deterministic drift.
4. Add browser-mediated SAFE fixes, backup history, and rollback.
5. Add sanitized ZIP export, public demo assets, documentation, and release gates.

Each phase remains independently testable and preserves all v0.1 functionality.
