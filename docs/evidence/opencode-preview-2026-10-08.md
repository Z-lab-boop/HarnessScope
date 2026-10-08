# OpenCode preview evidence snapshot

Checked on 2026-10-08 against OpenCode's official documentation.

- Configuration supports `opencode.json` and `opencode.jsonc`.
- Global configuration is documented under `~/.config/opencode/`.
- Project configuration is discovered from the working directory toward the nearest Git root.
- Project and global `AGENTS.md` files are documented instruction sources.
- `instructions` can reference local paths and glob patterns.

HarnessScope v0.1 intentionally exposes this adapter as `PREVIEW`. It inspects the bounded local sources above but does not fetch remote configuration or emit confirmed merge-precedence, shadowing, or effective-output edges.

Official references:

- https://opencode.ai/docs/config/
- https://opencode.ai/docs/rules/
