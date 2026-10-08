# Codex adapter evidence: 0.162.0-alpha.2

Verified on: 2026-10-08

The initial verified adapter is intentionally pinned to the locally observed `codex-cli 0.162.0-alpha.2`. Other versions retain syntax and source inspection but precedence-dependent conclusions are marked `COMPATIBILITY_UNKNOWN`.

## Primary references

- Configuration layers and precedence: https://learn.chatgpt.com/docs/config-file/config-basic
- `AGENTS.md` discovery, override filenames, root-to-CWD ordering, fallbacks, and size limit: https://learn.chatgpt.com/docs/agent-configuration/agents-md

## Committed evidence

- `fixtures/codex/basic` covers user/project TOML, global/root/nested instructions, `AGENTS.override.md`, one skill, MCP shadowing, and a synthetic credential canary.
- `fixtures/codex/malformed` covers a broken TOML source that does not abort unrelated inspection.
- `fixtures/codex/version-drift` verifies that an unrecognized client version suppresses confirmed precedence claims.

## Supported behavior

- Bounded system, user, project, and nested source discovery.
- TOML parsing for scalar settings, environment references, and MCP server entries.
- Global, root, and nested instruction discovery with override-file selection.
- One-level skill discovery for `SKILL.md` entries.
- Deterministic later-layer resolution with provenance edges.

## Explicit limitations

- Project `.codex/config.toml` layers are conditional on Codex project trust. Until trust state is directly established, these nodes and their override edges are labeled `LIKELY` with load condition `trusted_project`.
- CLI flags, one-shot `--config` values, and selected profiles are not available from a path-only scan and are not claimed as active.
- Cloud-managed values and enforced requirements that are not locally readable remain unknown.
- HarnessScope does not invoke an authenticated Codex session to validate runtime behavior.
