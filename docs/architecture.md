# Architecture

HarnessScope separates client-specific evidence from shared analysis:

```text
known local paths -> adapter discovery -> secret-safe parsing
                  -> client-local resolution -> normalized provenance graph
                  -> shared analyzers -> terminal / canonical JSON / offline HTML
                                     -> serialized service -> loopback dashboard
                                     -> sanitized snapshots -> normalized drift
                                     -> allowlisted diagnostic ZIP + SHA-256 manifest
```

## Trust boundaries

Adapters are the only components that know client-specific file locations, formats, and documented load rules. A verified adapter may emit confirmed precedence edges only for an exact tested client version. Preview adapters use `UNKNOWN` evidence for behavior-sensitive conclusions and currently emit no override, shadow, or effective-output edges.

Parsing converts raw values into `SafeValue` immediately. The shared model therefore carries display-safe values, not raw credential strings. Reports apply a second text scrub and collapse the scan root and home directory before serialization.

## Core packages

- `internal/adapters`: Codex, Claude Code, Cursor, and OpenCode implementations plus the registry contract.
- `internal/discovery`: bounded known-path inspection and ancestor walking; no broad home-directory crawl.
- `internal/model`: normalized sources, nodes, edges, findings, compatibility metadata, and versioned fix plans.
- `internal/resolver`: deterministic graph assembly and reference validation.
- `internal/analyzers`: duplicate MCP/context and missing-path checks, plus installed-version compatibility, captured-PATH bare commands, existing absolute-path portability, safe-value divergence, project credential presence and source symlink escape. Behavioral findings retain the weakest evidence; these are structural checks, not semantic interpretation.
- `internal/fixes`: narrow plan generation, source-hash checks, backups, atomic content replacement, postcondition verification, and rollback.
- `internal/report`: canonical JSON, terminal summaries, and a self-contained HTML report.
- `internal/server`: fixed workspace/home/app-data roots, immutable public state, monotonic revisions, serialized mutations, loopback HTTP and authenticated routes. Browser apply calls the same narrow fix engine.
- `internal/snapshots`: atomic user-only storage, strict names and schema-major reads, normalized fingerprints without raw values or paths.
- `internal/export`: deterministic allowlisted ZIP members for a fixed clock/input; member hashes and fail-before-write privacy checks.
- `web/dashboard`: bundled TypeScript/CSS, in-memory fragment token, same-origin API client, accessible workbench/inspector, graph, findings, compare, fix, drift and export views. No frontend runtime packages or CDN assets.

## Determinism and schemas

Stable IDs are hashes of normalized identities rather than traversal order. Canonicalization sorts clients, sources, graph objects, findings, context estimates, and fix plans. `schemas/report-v1.schema.json` and `schemas/fix-plan-v1.schema.json` reject unknown object properties in their stable structures.

The report schema and CLI version evolve independently. Readers accept schema major version 1 and reject unsupported majors.

v0.2 adds strict dashboard-state, drift and bundle-manifest schemas alongside the existing report/fix schemas. CI parses all five as strict JSON; package tests exercise serializers, schema-major boundaries, API strict-body decoding and unknown fields. Frontend bundles are rebuilt and compared with committed assets. Reproducible binary checks use `-trimpath -buildvcs=false`; archive timestamps are not promised byte-reproducible.

## Fix transaction

```text
scan -> plan -> rehash -> backup -> apply -> rescan/verify
                                      | failure
                                      v
                                   rollback
```

Backups live under the HarnessScope application-data directory with `0700` directories and `0600` files. A rollback restores only files listed in the selected transaction manifest.

## Browser boundary

The listener binds `127.0.0.1` on a selected or ephemeral port. Each server launch creates a random 256-bit session token. Static assets are public on loopback; every API route requires the custom token header, including reads. Host validation defends against DNS rebinding; mutations require the exact same origin, JSON content type, bounded body and current revision. Unknown fields and invalid route/query shapes are rejected. A single service lock covers revision checking, source-hash validation, mutation and state publication. Successful rescans/mutations advance revision once; failed operations do not publish partial state.

The token is printed once in a URL fragment, consumed into browser memory and removed from history. There are no cookies, local/session storage, CORS grants, remote redirects or external requests. CSP restricts resources to the embedded application; responses disable caching. SAFE selection and explicit confirmation are required; stale requests refresh state without replay. REVIEW/BLOCKED plans remain unapplied.

## Snapshot/export boundary

Only sanitized public state reaches snapshots or ZIP generation. Snapshot save replaces a validated name atomically; symlinks/nonregular files and unsupported schema majors are rejected. Drift compares an explicit structural field allowlist and stable IDs; raw attribute values, paths and timestamps are excluded. The ZIP contains README, report JSON/HTML, a manifest, and optional drift. Backups and raw source bytes are not diagnostic members. Users must still inspect a bundle before sharing; uncommon identifiers and paths outside known roots can require manual removal.

The dashboard is an explicitly refreshed snapshot, not a watcher. It has no telemetry or cloud component, does not launch configured MCP/hook commands, and only invokes installed client executables for version detection. See [release gates](release-gates.md) and [visual provenance](visual-baselines.md).

## Offline report

The HTML contains embedded CSS, JavaScript, and base64-encoded canonical JSON. Findings and the graph remain readable with JavaScript disabled; JavaScript only adds filtering and raw-data toggling. Playwright tests fail on non-file network requests.
