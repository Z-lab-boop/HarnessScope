# HarnessScope

HarnessScope is a local, offline inspector for coding-agent configuration. It answers three practical questions:

1. Which configuration and instruction files were discovered?
2. Where did an effective rule, MCP server, hook, or skill come from?
3. Which conflicts, missing paths, and duplicated context are actionable?

It currently supports verified adapters for Codex `0.162.0-alpha.2` and Claude Code `2.1.259`, plus conservative preview adapters for Cursor and OpenCode. Preview adapters discover documented local sources but do not claim confirmed precedence or shadowing.

## Five-minute start

Download the archive for your operating system and architecture from GitHub Releases, then verify it against `SHA256SUMS` before extracting:

```sh
shasum -a 256 -c SHA256SUMS
tar -xzf harnessscope_v0.1.0_darwin_arm64.tar.gz
./hscope scan . --open
```

Linux users can use `sha256sum -c SHA256SUMS`. Release archives contain the binary, license, notices, and both READMEs. Until a tagged release exists, build from source with Go 1.24 or newer:

```sh
go build -trimpath -o bin/hscope ./cmd/hscope
./bin/hscope scan . --open
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
```

Example summary from the committed synthetic demo:

```text
HarnessScope
- claude: VERIFIED VERIFIED (2.1.259)
- codex: VERIFIED VERIFIED (0.162.0-alpha.2)
- cursor: PREVIEW COMPATIBILITY_UNKNOWN (synthetic-preview)
- opencode: PREVIEW COMPATIBILITY_UNKNOWN (synthetic-preview)
Findings: HIGH=4 MEDIUM=2 LOW=0 INFO=0
```

Run the full demo with `./demo/run.sh`. It uses only synthetic configuration and checks that its credential canary never reaches terminal, JSON, or HTML output.

## Safety and privacy

- Scans are local and reports have no external assets or network requests.
- Secret-shaped fields and credential patterns are redacted immediately after parsing.
- Reports collapse the scanned root to `.` and the home directory to `~`.
- `fix` is a dry-run unless `--apply` is explicit.
- Automatic fixes are intentionally limited to three `SAFE` operations: exact duplicate-line removal in one source, executable-bit repair for an existing shebang hook, and canonical-path normalization to the same filesystem object.
- Every applied fix checks source hashes, creates user-only backups, writes atomically where content changes are involved, rescans, and automatically rolls back when verification fails.

No secret detector is perfect. Review a report before publishing it, especially when configuration contains unusual proprietary identifiers.

## Compatibility tiers

| Client | Tier | v0.1 boundary |
|---|---|---|
| Codex | VERIFIED | Exact ruleset verified against `0.162.0-alpha.2`; other versions are marked compatibility unknown. |
| Claude Code | VERIFIED | Exact ruleset verified against `2.1.259`; other versions are marked compatibility unknown. |
| Cursor | PREVIEW | Documented local rule and MCP sources; no confirmed effective-precedence claims. |
| OpenCode | PREVIEW | Documented local JSON/JSONC and instruction sources; remote config is not fetched and effective merge precedence is not asserted. |

## Limitations and roadmap

HarnessScope does not execute MCP servers, contact remote configuration endpoints, infer undisclosed client internals, or treat preview behavior as verified. Context token counts are bounded estimates rather than provider billing values.

The next milestones are broader fixture-backed version coverage, schema migration tooling, richer graph navigation, and opt-in support for additional agent clients. Evidence snapshots for current adapter claims live under `docs/evidence/`.

See [architecture](docs/architecture.md), [contributing](CONTRIBUTING.md), and [security policy](SECURITY.md). The project is licensed under Apache-2.0.

## 中文

中文说明见 [README.zh-CN.md](README.zh-CN.md)。
