# HarnessScope

HarnessScope v0.2 is a local, offline inspector and browser dashboard for coding-agent configuration. It answers three practical questions:

1. Which configuration and instruction files were discovered?
2. Where did an effective rule, MCP server, hook, or skill come from?
3. Which conflicts, missing paths, and duplicated context are actionable?

It currently supports verified adapters for Codex `0.162.0-alpha.2` and Claude Code `2.1.259`, plus conservative preview adapters for Cursor and OpenCode. Preview adapters discover documented local sources but do not claim confirmed precedence or shadowing.

## Five-minute start

Download the archive for your operating system and architecture from GitHub Releases, then verify it against `SHA256SUMS` before extracting:

```sh
shasum -a 256 -c SHA256SUMS
tar -xzf harnessscope_v0.2.0_darwin_arm64.tar.gz
./hscope serve . --port 0 --open
```

Linux users can use `sha256sum -c SHA256SUMS`. Archives target darwin/linux × amd64/arm64 and contain the binary, license, notices, security policy, docs and both READMEs. Local candidate archives do not imply a published release. Until a tagged release exists, build from source with Go 1.24 or newer:

```sh
go build -trimpath -o bin/hscope ./cmd/hscope
./bin/hscope serve . --port 0 --open
```

By default, JSON and self-contained HTML reports are written under the platform application-data directory, not into the scanned repository. Use `--no-report` for terminal-only inspection.

## Commands

```text
hscope scan [path] [--client all|codex|claude|cursor|opencode]
hscope explain <normalized-name> [--client ...] [--path ...]
hscope compare <client-a> <client-b> [--path ...]
hscope report --from report.json --html report.html
hscope fix [fix-id...]              # dry-run by default
hscope fix [fix-id...] --apply      # SAFE fixes only
hscope rollback <backup-id>
hscope serve [path] [--port 0] [--open] [--client ...]
hscope snapshot save <name> [path]
hscope snapshot list
hscope snapshot diff <name> [path]
hscope export [path] --output diagnostic.zip [--baseline <name>] [--force]
```

## Local control center

`serve` binds only to `127.0.0.1`; port 0 selects an available port. Open the authenticated URL printed once on startup. Its fragment token is consumed into memory and removed from the address bar; reloading requires opening the original launch URL again. Ctrl-C shuts the server down. There is no remote access, cloud service, telemetry or background live monitoring. Rescan explicitly refreshes the fixed workspace snapshot.

Overview shows client evidence and risk summaries; Graph exposes provenance and an inspector; Findings filters structural checks; Compare shows normalized declarations. In Fix Center, select SAFE plans, inspect the exact preview and confirm application. REVIEW and BLOCKED plans cannot be applied from the browser. A stale revision refreshes state and requires an explicit retry. Backups and rollback remain local.

![Synthetic dashboard overview](docs/assets/dashboard-overview.png)

Illustrative macOS Chromium capture of the committed Playwright synthetic fixture; not real user configuration and not a Linux visual regression baseline. [Visual provenance and open Linux release gate](docs/visual-baselines.md).

Drift saves named, sanitized local baselines and compares normalized identities and fingerprints. Names use 1–64 ASCII letters, digits, dots, underscores or hyphens, starting with a letter/digit; saving an existing name replaces that baseline. Drift intentionally omits raw before/after values, paths and timestamps and is not a semantic diff. Baselines are stored in platform application data, with user-only permissions.

Export writes `report.json`, self-contained `report.html`, `README.txt`, `manifest.json` and optionally `drift.json`. The manifest records client tiers, tool/schema versions, generation time and member SHA-256 hashes. It excludes raw configurations, secrets, backups and session tokens. Browser downloads remain local; CLI refuses an existing output unless `--force` is explicit. Review all contents before uploading publicly. Sanitization is a defense, not a guarantee for arbitrary proprietary formats.

Run `./demo/run.sh` for noninteractive assertions or `./demo/dashboard.sh` for the disposable browser demo. Both copy committed synthetic configuration to temporary state and refuse existing managed agent configuration. The assertions cover duplicate MCP/context, missing paths, unsupported client versions, unavailable bare commands, nonportable paths, divergent declarations, project secret presence and escaping symlinks, then snapshot/drift/ZIP round trips and credential/path leak checks. Only the noninteractive demo retains temporary artifacts for inspection.

## Safety and privacy

- Scans are local and reports have no external assets or network requests.
- Secret-shaped fields and credential patterns are redacted immediately after parsing.
- Reports collapse the scanned root to `.` and the home directory to `~`.
- `fix` is a dry-run unless `--apply` is explicit.
- Automatic fixes are intentionally limited to three `SAFE` operations: exact duplicate-line removal in one source, executable-bit repair for an existing shebang hook, and canonical-path normalization to the same filesystem object.
- Every applied fix checks source hashes, creates user-only backups, writes atomically where content changes are involved, rescans, and automatically rolls back when verification fails.

No secret detector is perfect. Review a report before publishing it, especially when configuration contains unusual proprietary identifiers.

## Compatibility tiers

| Client | Tier | v0.2 boundary |
|---|---|---|
| Codex | VERIFIED | Exact ruleset verified against `0.162.0-alpha.2`; other versions are marked compatibility unknown. |
| Claude Code | VERIFIED | Exact ruleset verified against `2.1.259`; other versions are marked compatibility unknown. |
| Cursor | PREVIEW | Documented local rule and MCP sources; no confirmed effective-precedence claims. |
| OpenCode | PREVIEW | Documented local JSON/JSONC and instruction sources; remote config is not fetched and effective merge precedence is not asserted. |

## Limitations and roadmap

HarnessScope does not execute MCP servers, contact remote configuration endpoints, infer undisclosed client internals, or treat preview behavior as verified. Context token counts are bounded estimates rather than provider billing values.

Checks are structural and evidence-bounded; HarnessScope does not understand instruction semantics or guarantee client runtime behavior. Pan/zoom persistence across graph filter rerenders is deferred. Evidence snapshots for current adapter claims live under `docs/evidence/`. See [release gates](docs/release-gates.md) for reproducible local checks and the currently open reviewed-Linux visual gate. Repository creation, push and release publication are separate operator actions.

See [architecture](docs/architecture.md), [contributing](CONTRIBUTING.md), and [security policy](SECURITY.md). The project is licensed under Apache-2.0.

## 中文

中文说明见 [README.zh-CN.md](README.zh-CN.md)。
