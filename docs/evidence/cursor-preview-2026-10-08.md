# Cursor preview evidence snapshot

Checked on 2026-10-08 against Cursor's official documentation.

- Project rules use `.cursor/rules/*.mdc`; nested `.cursor/rules` directories are supported.
- Legacy `.cursorrules` remains supported but is deprecated.
- MCP configuration is documented at `~/.cursor/mcp.json` and project `.cursor/mcp.json`.
- Global and project MCP files are merged, with project declarations taking priority for a repeated server name.

HarnessScope v0.1 intentionally exposes this adapter as `PREVIEW`. It discovers and parses these documented local sources but does not emit confirmed precedence, shadowing, or effective-output edges.

Official references:

- https://cursor.com/docs/context/rules
- https://cursor.com/help/customization/mcp
