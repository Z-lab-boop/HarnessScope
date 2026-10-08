# Architecture

HarnessScope separates client-specific evidence from shared analysis:

```text
known local paths -> adapter discovery -> secret-safe parsing
                  -> client-local resolution -> normalized provenance graph
                  -> shared analyzers -> terminal / canonical JSON / offline HTML
```

## Trust boundaries

Adapters are the only components that know client-specific file locations, formats, and documented load rules. A verified adapter may emit confirmed precedence edges only for an exact tested client version. Preview adapters use `UNKNOWN` evidence for behavior-sensitive conclusions and currently emit no override, shadow, or effective-output edges.

Parsing converts raw values into `SafeValue` immediately. The shared model therefore carries display-safe values, not raw credential strings. Reports apply a second text scrub and collapse the scan root and home directory before serialization.

## Core packages

- `internal/adapters`: Codex, Claude Code, Cursor, and OpenCode implementations plus the registry contract.
- `internal/discovery`: bounded known-path inspection and ancestor walking; no broad home-directory crawl.
- `internal/model`: normalized sources, nodes, edges, findings, compatibility metadata, and versioned fix plans.
- `internal/resolver`: deterministic graph assembly and reference validation.
- `internal/analyzers`: cross-client duplicate MCP, missing-path, and duplicate-context checks.
- `internal/fixes`: narrow plan generation, source-hash checks, backups, atomic content replacement, postcondition verification, and rollback.
- `internal/report`: canonical JSON, terminal summaries, and a self-contained HTML report.

## Determinism and schemas

Stable IDs are hashes of normalized identities rather than traversal order. Canonicalization sorts clients, sources, graph objects, findings, context estimates, and fix plans. `schemas/report-v1.schema.json` and `schemas/fix-plan-v1.schema.json` reject unknown object properties in their stable structures.

The report schema and CLI version evolve independently. Readers accept schema major version 1 and reject unsupported majors.

## Fix transaction

```text
scan -> plan -> rehash -> backup -> apply -> rescan/verify
                                      | failure
                                      v
                                   rollback
```

Backups live under the HarnessScope application-data directory with `0700` directories and `0600` files. A rollback restores only files listed in the selected transaction manifest.

## Offline report

The HTML contains embedded CSS, JavaScript, and base64-encoded canonical JSON. Findings and the graph remain readable with JavaScript disabled; JavaScript only adds filtering and raw-data toggling. Playwright tests fail on non-file network requests.
