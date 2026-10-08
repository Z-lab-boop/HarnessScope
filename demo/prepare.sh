#!/bin/sh
# Sourced only by the demo launchers. All mutable state is in a fresh temp root.
prepare_demo() {
  # Adapters inspect these managed locations even with a synthetic HOME.
  for managed in /etc/codex '/Library/Application Support/ClaudeCode/managed-settings.json' /etc/claude-code/managed-settings.json; do
    if [ -e "$managed" ]; then
      printf 'Synthetic demo refuses existing managed configuration: %s\n' "$managed" >&2
      return 1
    fi
  done
  cp -R "$repository/demo/conflicted-workspace/." "$output/fixture"
  demo="$output/fixture"
  mkdir -p "$demo/workspace/.git" "$demo/workspace/.claude" "$demo/home/outside"
  mv "$demo/bin" "$demo/home/bin"
  # Templates reference only this disposable synthetic workspace.
  sed "s|__WORKSPACE__|$demo/workspace|g" "$demo/project-settings.json.in" >"$demo/workspace/.claude/settings.json"
  cp "$demo/escaped-mcp.json" "$demo/home/outside/mcp.json"
  ln -s ../home/outside/mcp.json "$demo/workspace/.mcp.json"
}

demo_hscope() {
  HOME="$demo/home" XDG_DATA_HOME="$demo/home/.local/share" PATH="$demo/home/bin:/usr/bin:/bin" "$binary" "$@"
}
