# Claude Code adapter evidence: 2.1.259

Verified on: 2026-10-08

The initial verified adapter is intentionally pinned to the locally observed `2.1.259 (Claude Code)`. Other versions retain syntax and source inspection but precedence-dependent conclusions are marked `COMPATIBILITY_UNKNOWN`.

## Primary references

- Memory, `CLAUDE.md`, imports, nested loading, and version-specific behavior: https://code.claude.com/docs/en/memory
- User, project, local, and managed settings: https://code.claude.com/docs/en/settings
- User, project, and local MCP scopes: https://code.claude.com/docs/en/mcp

## Committed evidence

- `fixtures/claude/basic` covers user/project/local settings, user and project memory, one recursive import, hooks, skills, project MCP, and a synthetic credential canary.
- `fixtures/claude/imports` covers a direct recursive import cycle.
- `fixtures/claude/malformed` covers invalid JSON without aborting unrelated inspection.
- `fixtures/claude/version-drift` verifies that an unrecognized client version suppresses confirmed precedence claims.

## Supported behavior

- Bounded user, project, local, and managed source discovery.
- JSON parsing for scalar settings, environment references, hooks, and MCP servers.
- Root-to-working-directory `CLAUDE.md` discovery.
- `@path` imports outside fenced and inline code, with cycle detection and a four-hop limit.
- One-level user and project skill discovery.
- Deterministic later-layer resolution with provenance edges.

## Version-dependent exclusions and limitations

- Direct `AGENTS.md` loading is not claimed for 2.1.259. The official documentation identifies it as a later feature, so this adapter inspects `CLAUDE.md` sources only.
- Project mappings inside `~/.claude.json` are not interpreted in v0.1; raw values from that file never cross the redaction boundary.
- External imports may require runtime approval. HarnessScope reports the source relationship but does not claim that a user approved it.
- CLI flags and `--settings` inputs are invocation-specific and are not claimed by a path-only scan.
- HarnessScope does not start an authenticated Claude Code session to validate runtime behavior.
