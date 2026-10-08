# Expected demo signals

The synthetic demo must produce these rule IDs:

- `MCP-0001`: `mcp.shared` is declared by more than one client/source.
- `PATH-0001`: `/harnessscope-demo/missing-server` does not exist.
- `CONTEXT-0001`: the same fixed instruction text is present in multiple sources.
- `CLIENT-0001`: synthetic Cursor/OpenCode versions have no verified ruleset.
- `CMD-0001`: synthetic bare MCP commands are absent from the captured PATH.
- `PATH-0002`: the rendered MCP working directory is an existing absolute path within the disposable workspace.
- `CONFIG-0001`: declarations of `mcp.shared` have different safe values.
- `SECRET-0001`: a credential-shaped project field exists; only category/presence is exposed.
- `SOURCE-0004`: a project MCP symlink resolves into a synthetic sibling directory outside the workspace root.

The report must identify Codex and Claude Code as `VERIFIED`, Cursor and OpenCode as `PREVIEW`, and must not contain the literal `HARNESSSCOPE-CANARY` credential marker.

`demo/prepare.sh` copies these committed inputs into a fresh temporary directory, renders the project template there and creates the synthetic symlink. It refuses hosts with managed Codex/Claude configuration, isolates HOME/XDG/PATH and places a synthetic project boundary. It never edits the committed fixture or real user configuration. The noninteractive demo checks an unchanged baseline, then adds a synthetic declaration, verifies ADDED drift, exports and extracts a ZIP, checks every expected member and rejects credential/root leaks. Output is retained at the printed temporary paths for inspection.

`demo/dashboard.sh` uses the same preparation, prints the one authenticated port-0 launch URL, and removes its temporary state on Ctrl-C. Browser fixes therefore modify only disposable copies. The public screenshot uses the separate committed Playwright fixture; see `docs/visual-baselines.md`.
