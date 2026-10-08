#!/bin/sh
set -eu

if [ "$#" -ne 1 ]; then
  printf 'usage: %s /path/to/hscope\n' "$0" >&2
  exit 2
fi

case "$1" in
  /*) binary=$1 ;;
  *) binary=$(CDPATH= cd -- "$(dirname -- "$1")" && pwd)/$(basename -- "$1") ;;
esac
smoke_root=$(mktemp -d "${TMPDIR:-/tmp}/harnessscope-smoke.XXXXXX")
trap 'rm -rf "$smoke_root"' EXIT INT TERM

mkdir -p "$smoke_root/home/.cursor" "$smoke_root/bin" "$smoke_root/workspace"
printf '%s\n' '#!/bin/sh' "printf '%s\\n' 'cursor smoke-preview'" >"$smoke_root/bin/cursor"
chmod 700 "$smoke_root/bin/cursor"
cat >"$smoke_root/home/.cursor/mcp.json" <<'EOF'
{
  "mcpServers": {
    "smoke": {
      "command": "/harnessscope-smoke/missing",
      "env": { "API_KEY": "sk-smoke-HARNESSSCOPE-CANARY-1234567890" }
    }
  }
}
EOF

"$binary" --help >/dev/null
HOME="$smoke_root/home" PATH="$smoke_root/bin:$PATH" "$binary" scan "$smoke_root/workspace" --client cursor --no-report >"$smoke_root/output.txt"
grep -q 'PREVIEW' "$smoke_root/output.txt"
grep -q 'PATH-0001' "$smoke_root/output.txt"
if grep -q 'HARNESSSCOPE-CANARY' "$smoke_root/output.txt"; then
  printf '%s\n' 'canary leaked during release smoke test' >&2
  exit 1
fi
