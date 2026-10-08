# HarnessScope v0.1 Design Specification

**Status:** Approved for implementation
**Date:** 2026-10-08  
**Product:** HarnessScope  
**CLI:** `hscope`  
**Tagline:** See what your coding agent actually loads. / 看清编码助手真正加载了什么。

## 1. Summary

HarnessScope is a local, offline-first configuration inspector for AI coding assistants. Given a working directory, it discovers the relevant user, project, and nested configuration sources for Codex, Claude Code, Cursor, and OpenCode. Verified adapters resolve client-specific loading and precedence rules and explain the resulting effective configuration with file- and field-level evidence; preview adapters provide only the explicitly supported subset of that analysis.

The product is not a generic runtime doctor. Its defining capability is effective-configuration provenance: it answers what a client will load in this directory, why each item is effective, what was shadowed or ignored, and how much fixed context the configuration is likely to consume.

The v0.1 release targets individual developers on macOS and Linux. Core analysis is deterministic and requires no model API, account, telemetry, or network connection. Configuration changes are opt-in, previewed, backed up, applied atomically, and verified by rescanning.

## 2. Market Position

Existing projects named Agent Doctor already diagnose runtimes, validate instruction files, or repair local agent installations. AgentRC and AgentSync generate or synchronize agent configuration. HarnessScope deliberately avoids those crowded categories.

HarnessScope occupies a narrower position:

> `git config --show-origin`, but for the complete configuration and context surface of AI coding assistants.

Its differentiators are:

1. Client-accurate source discovery and precedence resolution.
2. A graph of load, import, override, shadow, duplicate, and reference relationships.
3. Cross-client comparison from the same working directory.
4. Evidence-bearing findings rather than a vague health score.
5. Offline reports and secret-safe fix plans.

## 3. Target User and Primary Job

The v0.1 target user is an individual developer who uses two or more coding assistants on one machine and has accumulated global and repository-level instructions, skills, hooks, and MCP registrations.

The primary job is:

> Before starting work in this directory, show me exactly what each coding assistant will load, where it came from, which configuration wins, what is broken or redundant, and what I can safely change.

The primary success path must complete in five minutes:

```bash
hscope scan . --open
```

## 4. Goals

- Resolve the effective configuration for supported clients at a specified working directory.
- Preserve provenance from every effective value back to its source file and field.
- Explain overrides and ignored sources using client-specific precedence rules.
- Detect high-confidence structural, path, MCP, hook, rule, context, and portability issues.
- Compare the effective configuration of multiple clients.
- Generate concise terminal output, stable JSON, and a self-contained offline HTML report.
- Offer conservative, reversible fixes without silently changing configuration.
- Prevent credentials and other secret values from entering reports, logs, backups metadata, fixtures, or crash output.
- Make unsupported client versions and incomplete evidence visible rather than guessing.

## 5. Non-goals

The v0.1 release will not:

- Validate API keys by contacting providers.
- Start, stop, repair, or supervise agent processes.
- Synchronize one canonical configuration into multiple clients.
- Read chat transcripts or session histories.
- Generate instructions, skills, or MCP servers.
- Depend on an LLM for core findings.
- Claim exact token counts where no authoritative local tokenizer is available.
- Provide Windows support.
- Provide organization policy, fleet management, hosted dashboards, or telemetry.
- Produce a single health score that obscures individual evidence.

## 6. Supported Clients and Confidence Tiers

### 6.1 Verified adapters

Codex and Claude Code receive full v0.1 adapters. A verified adapter supports:

- installation and version discovery;
- known global, project, and nested source discovery;
- instruction, skill, hook, and MCP parsing where applicable;
- documented precedence and load-condition resolution;
- source-level and field-level provenance;
- golden fixtures for supported versions.

### 6.2 Preview adapters

Cursor and OpenCode receive preview adapters. Preview support includes source discovery, syntax checks, MCP inspection, and path validation. A preview adapter must label its output `PREVIEW` and must not make unsupported precedence claims.

### 6.3 Version drift policy

Each adapter ships metadata containing:

- verified client version range;
- ruleset version;
- last verification date;
- evidence references;
- supported feature list.

When the installed client is outside the verified range, HarnessScope continues deterministic parsing but marks precedence-dependent conclusions `COMPATIBILITY_UNKNOWN`. It never silently treats a newer client as behaviorally identical.

## 7. User Interface and Commands

### 7.1 Scan

```bash
hscope scan [path]
hscope scan . --client codex
hscope scan . --client all --json report.json --html report.html
hscope scan . --open
```

`scan` discovers installed clients, resolves known sources, runs analyzers, and prints a compact summary. By default it writes JSON and HTML to the operating system's per-user HarnessScope application-data directory rather than modifying the scanned repository. The terminal prints the resulting paths. The user may supply alternate paths, disable reports with `--no-report`, or open the HTML result with `--open`.

Generating reports does not count as modifying client configuration. No client configuration is changed during `scan`.

### 7.2 Explain

```bash
hscope explain mcp.playwright --client codex
hscope explain instruction.root --client claude
```

`explain` prints the complete provenance chain for one normalized configuration element, including sources considered, load conditions, overrides, the effective value, and any uncertainty.

### 7.3 Compare

```bash
hscope compare codex claude cursor
```

`compare` highlights elements that differ across clients, including missing rules, divergent MCP launchers, different instruction scopes, and estimated fixed-context cost.

### 7.4 Report

```bash
hscope report --from <report.json> --html report.html
```

`report` renders a previously captured scan without rereading live configuration. This separates analysis from presentation and makes bug reports reproducible.

### 7.5 Fix and rollback

```bash
hscope fix --dry-run
hscope fix --apply
hscope fix --apply FIX-MCP-0004
hscope rollback <backup-id>
```

`fix --dry-run` is the default behavior and prints structured patches. `fix --apply` applies only findings classified `SAFE`. Findings classified `REVIEW` require an explicit fix ID and an interactive confirmation. Findings classified `MANUAL` are never applied by HarnessScope.

Every applied plan creates a backup manifest, writes atomically, rescans affected sources, and rolls back automatically if parsing or the intended postcondition fails.

### 7.6 Exit codes

- `0`: command completed and no configured failure threshold was crossed;
- `1`: `--fail-on` threshold crossed;
- `2`: operational error such as unreadable input or unusable report;
- `3`: requested fix or rollback could not be completed safely.

The default `scan` threshold is `none`, so findings do not turn an otherwise successful local scan into an error unless the user opts in.

## 8. Architecture

HarnessScope is a Go command-line application with an embedded, self-contained HTML report frontend.

```text
CLI
 ├── Client Discovery
 ├── Source Discovery
 ├── Client Adapters
 ├── Effective Config Resolver
 ├── Analyzers
 ├── Fix Planner and Transaction Runner
 └── Terminal / JSON / HTML Reporters
```

### 8.1 Package boundaries

```text
cmd/hscope/                 CLI entry point
internal/discovery/         executable, version, OS, and source discovery
internal/adapters/          isolated client adapters
  codex/
  claude/
  cursor/
  opencode/
internal/model/             normalized source, node, edge, finding, and fix types
internal/resolver/          graph construction and effective-value resolution
internal/analyzers/         deterministic diagnostic rules
internal/secrets/           early redaction and secret-safe value handling
internal/fixes/             plan, backup, atomic apply, verify, rollback
internal/report/            terminal and JSON output
web/                        TypeScript/SVG report source
internal/report/assets/     compiled and embedded offline assets
fixtures/                   synthetic adapter and regression fixtures
schemas/                    stable JSON schemas for reports and fix plans
```

### 8.2 Adapter interface

```go
type ClientAdapter interface {
    Detect(ctx context.Context) DetectionResult
    DiscoverSources(ctx context.Context, cwd string) []ConfigSource
    Parse(ctx context.Context, source ConfigSource) ParsedConfig
    Resolve(ctx context.Context, sources []ParsedConfig) EffectiveConfig
    Capabilities() AdapterCapabilities
    Compatibility() CompatibilityMetadata
}
```

Adapters own client-specific paths, parsing extensions, load conditions, and precedence. Shared analyzers operate only on normalized models. A client format change therefore remains isolated to one adapter and its fixtures.

### 8.3 Normalized graph

The resolver produces a directed graph with these node classes:

- client;
- scope;
- source document;
- instruction;
- rule;
- skill;
- hook;
- MCP server;
- environment reference;
- effective output.

Edges use explicit types:

- `LOADS`;
- `IMPORTS`;
- `OVERRIDES`;
- `SHADOWS`;
- `DUPLICATES`;
- `REFERENCES`;
- `EFFECTIVE_AS`.

Every edge includes the adapter ruleset and evidence that produced it. Stable node IDs derive from client, normalized source identity, and field path. Home paths are displayed with `~` in reports to avoid unnecessary machine-specific disclosure.

## 9. Core Data Model

### 9.1 ConfigSource

Contains logical path, canonical path, client, scope, format, existence, readability, source kind, discovery reason, and symlink information.

HarnessScope scans only documented or adapter-declared locations. It does not recursively crawl the entire home directory.

### 9.2 ConfigNode

Contains normalized type, stable ID, display name, redacted attributes, source location, field path, load condition, and adapter confidence.

### 9.3 Origin

Contains source ID, line and column when available, structured field path, scope, precedence rank, and the rule that caused the source to be considered.

### 9.4 Finding

Contains rule ID, severity, evidence status, summary, impact, affected clients, origins, graph references, remediation, and optional fix-plan reference.

### 9.5 FixPlan

Contains targeted source hashes, preconditions, ordered edits, risk class, backup requirements, postconditions, rollback operations, and redacted user-facing diffs.

## 10. Diagnostics

### 10.1 Severity

- `HIGH`: configuration is broken, exposes a secret, or materially changes intended behavior;
- `MEDIUM`: likely waste, portability failure, shadowing, or maintainability risk;
- `LOW`: minor inconsistency with bounded impact;
- `INFO`: useful provenance or optimization opportunity.

### 10.2 Evidence status

- `CONFIRMED`: deterministic parsing or filesystem evidence proves the conclusion;
- `LIKELY`: a bounded heuristic supports the conclusion and human review is required;
- `UNKNOWN`: version drift, missing permissions, or incomplete source data prevents a conclusion.

Severity and evidence are independent. A high-impact issue may still be `UNKNOWN`; the report must not silently promote it to confirmed.

### 10.3 Rule families

| Family | Scope |
|---|---|
| `PARSE` | malformed JSON, JSONC, TOML, YAML, Markdown metadata, or unsupported syntax |
| `SOURCE` | present but unreachable, ignored, unreadable, cyclic, or version-unknown sources |
| `OVERRIDE` | user, project, nested, imported, or local configuration shadowing |
| `MCP` | duplicate names, divergent launchers, missing commands, invalid working directories, unpinned remote execution |
| `PATH` | missing, non-executable, machine-specific, or portability-sensitive paths |
| `ENV` | missing references, literal secret candidates, unsafe report exposure |
| `RULE` | duplicate instructions, unreachable rules, scope mismatch, explicit deterministic contradictions |
| `HOOK` | missing scripts, non-executable scripts, direct cycles, and known recursive invocation patterns |
| `CONTEXT` | duplicate fixed context, oversized sources, and unusually large MCP schemas |
| `PORTABLE` | macOS/Linux shell, path, case, executable, and line-ending incompatibilities |

Natural-language semantic contradiction detection is limited to explicit deterministic patterns in v0.1. General semantic judgment belongs to a later optional AI plugin and must never be presented as deterministic core analysis.

## 11. Context Cost Estimation

HarnessScope reports source bytes, Unicode code-point counts, and a conservative token range. It uses a bundled local tokenizer only where the encoding is known and supported. Otherwise it labels the result `ESTIMATED` and reports a range rather than a false exact count.

Context reporting distinguishes:

- always-loaded instructions;
- conditionally loaded rules and nested instructions;
- MCP tool descriptions and schemas;
- lazy skills;
- duplicated content;
- unknown client-owned system context.

The report never claims to reconstruct a provider's hidden system prompt.

## 12. Secret and Privacy Model

HarnessScope has no network code in v0.1. It performs no telemetry and does not check for updates automatically.

Secret handling occurs immediately after parsing and before normalized nodes, findings, logs, or reports are constructed.

Rules include common credential field names, token formats, private-key blocks, authorization headers, and high-entropy literal candidates. Raw values may be examined transiently in process memory but are never stored in result objects. Reports retain only the field name, secret category, source location, and `[REDACTED]`.

Environment checks test presence only. Environment values are never copied into reports.

Crash messages and debug logs use the same redaction layer. Fixtures use synthetic canary secrets and CI fails if any canary reaches terminal, JSON, HTML, backup metadata, or error output.

## 13. Fix Safety

### 13.1 Risk classes

- `SAFE`: bounded mechanical change with an unambiguous postcondition;
- `REVIEW`: behavior-affecting change requiring a named selection and confirmation;
- `MANUAL`: insufficient information or semantic judgment required.

Initial `SAFE` operations are intentionally narrow:

- remove byte-identical duplicates inside the same source and same scope;
- set the executable bit on an existing local hook script with a valid shebang;
- normalize a path only when the existing target and normalized target resolve to the same canonical object.

Cross-scope deletion, MCP command changes, instruction movement, and path replacement are at least `REVIEW`.

### 13.2 Transaction protocol

1. Hash and reparse every targeted source.
2. Refuse if a source changed since the plan was created.
3. Create user-only backups and a redacted manifest.
4. Write replacement files beside the originals.
5. Flush, preserve applicable permissions, and atomically rename.
6. Reparse all affected sources.
7. Rescan intended postconditions.
8. Roll back automatically on failure.

Rollback restores only files in the selected transaction. HarnessScope never deletes unrelated backups or configuration.

## 14. Reports

### 14.1 Terminal

The terminal report shows detected clients, compatibility tier, counts by severity and evidence, then actionable findings. Each finding includes source location, reason, impact, and next action.

### 14.2 JSON

The JSON schema is versioned independently from the CLI. It contains normalized clients, sources, graph, findings, compatibility metadata, context estimates, and fix references. Secret values are structurally impossible in the schema.

Deterministic analysis data is separated from optional run metadata so identical fixtures produce byte-stable canonical JSON.

### 14.3 HTML

The HTML report is a single offline file with compiled TypeScript, CSS, and SVG assets inlined. It includes:

- overview and compatibility notices;
- effective-configuration graph;
- source and override chain;
- cross-client comparison matrix;
- context-cost distribution;
- filterable findings;
- redacted fix previews.

It uses no CDN, remote font, analytics, or remote image. The report remains useful without JavaScript through a generated summary and findings table.

## 15. Error Handling

One broken source does not abort the entire scan. The adapter records a `PARSE` or `SOURCE` finding, preserves other results, and marks dependent conclusions `UNKNOWN`.

Symlinks retain both logical and canonical paths. Discovery follows only known configuration links, detects cycles, and never expands into an unrestricted home-directory crawl.

Missing clients are skipped rather than failed. Missing optional permissions produce partial results with an explicit limitation. Unsupported versions retain syntax-level results but suppress unsupported precedence claims.

Operational failures include a concise terminal message and a redacted machine-readable error when JSON output is requested.

## 16. Testing Strategy

### 16.1 Adapter fixtures

Each adapter has synthetic global, project, nested, imported, symlinked, malformed, and version-drift fixtures. Golden outputs cover source discovery, graph edges, effective values, and evidence labels.

### 16.2 Isolation

Integration tests create temporary home directories and workspaces. No test may read or modify the developer's real agent configuration.

### 16.3 Property and fuzz tests

Parsers, path normalization, graph cycle detection, secret redaction, and report serialization receive fuzz or property tests. Required invariants include no raw secret propagation and stable canonical JSON.

### 16.4 Fix round trips

Every supported fix runs this sequence:

```text
scan -> plan -> apply -> rescan -> rollback -> rescan
```

Tests verify intended change, unrelated-byte preservation where the format permits it, successful rollback, and refusal after concurrent modification.

### 16.5 Report tests

HTML tests verify offline assets, embedded data, escaping, keyboard navigation, color-independent severity cues, and readable no-JavaScript fallback. Representative pages receive screenshot regression checks.

### 16.6 Release verification

CI runs unit tests, race detection, static analysis, adapter golden tests, isolated end-to-end scans, redaction canaries, frontend build, and artifact smoke tests on macOS and Linux. Release archives are unpacked into clean temporary directories and executed before publication.

## 17. Repository and Delivery

Implementation will live in an independent Git repository named `harnessscope`, separate from the existing innovation-project repository. The planned local checkout is `/Users/zzz/Documents/ChatGPT/HarnessScope`; creation of that checkout happens only after written-spec approval.

The repository will include:

- Apache-2.0 license;
- English primary README with a complete Chinese README;
- `SECURITY.md`, `CONTRIBUTING.md`, and architecture documentation;
- checksummed macOS and Linux release archives for amd64 and arm64;
- a deliberately conflicted synthetic demo workspace;
- a 15-second terminal-to-graph demonstration;
- no real user configuration or unreviewed sensitive fixture.

The initial installation path is a checksummed GitHub Release archive. Homebrew packaging follows after the first verified release rather than blocking v0.1.

## 18. Four-Week v0.1 Plan Boundary

### Week 1

Build the normalized model, discovery layer, adapter contract, secret-safe value types, and initial Codex and Claude fixtures.

### Week 2

Implement verified Codex and Claude resolution, rule engine, graph construction, and `scan`, `explain`, and `compare`.

### Week 3

Implement JSON schema, self-contained HTML report, conservative context estimates, fix planning, backup, apply, verification, and rollback.

### Week 4

Add Cursor and OpenCode preview adapters, harden redaction and error handling, run isolated dogfood fixtures, produce documentation and demo assets, and verify release archives.

Features outside this boundary move to v0.2 rather than weakening v0.1 evidence quality.

## 19. Acceptance Criteria

HarnessScope v0.1 is ready for public release only when all of the following are true:

1. A fresh user can generate the first offline HTML report within five minutes.
2. Verified adapters correctly resolve all committed precedence fixtures.
3. Every finding has a rule ID, evidence status, source location, reason, impact, and remediation.
4. Unsupported client versions are visibly downgraded rather than silently assumed compatible.
5. Canary secrets never appear in terminal, JSON, HTML, logs, errors, or backup metadata.
6. Every supported automatic fix passes the full apply, rescan, rollback, and rescan round trip.
7. Reports work without network access and remain readable without JavaScript.
8. Release archives pass clean-directory smoke tests on supported operating systems and architectures.
9. Public claims are limited to behavior demonstrated by committed fixtures and release verification.
10. The repository contains no copied third-party implementation without compatible licensing and attribution.

## 20. Deferred Roadmap

The following are explicitly deferred:

- Windows adapter and packaging;
- GitHub Action and repository-maintainer mode;
- optional local or cloud AI explanation plugin;
- semantic contradiction analysis;
- organization policy and fleet comparison;
- signed reports and SBOM-based trust metadata;
- Homebrew and other package managers;
- additional clients such as Gemini CLI, Copilot CLI, Windsurf, and Aider.

Each deferred capability must preserve the v0.1 guarantees of provenance, evidence labeling, offline core behavior, and secret-safe output.
