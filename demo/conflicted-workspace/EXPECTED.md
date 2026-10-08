# Expected demo signals

The synthetic demo must produce these rule IDs:

- `MCP-0001`: `mcp.shared` is declared by more than one client/source.
- `PATH-0001`: `/harnessscope-demo/missing-server` does not exist.
- `CONTEXT-0001`: the same fixed instruction text is present in multiple sources.

The report must identify Codex and Claude Code as `VERIFIED`, Cursor and OpenCode as `PREVIEW`, and must not contain the literal `HARNESSSCOPE-CANARY` credential marker.
