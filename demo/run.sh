#!/bin/sh
set -eu

repository=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
output=$(mktemp -d "${TMPDIR:-/tmp}/harnessscope-demo.XXXXXX")
output=$(CDPATH= cd -- "$output" && pwd -P)
binary="$output/hscope"
. "$repository/demo/prepare.sh"
prepare_demo

(
  cd "$repository"
  go build -trimpath -o "$binary" ./cmd/hscope
)

demo_hscope scan "$demo/workspace" \
  --json "$output/report.json" --html "$output/report.html" >"$output/terminal.txt"

for rule in MCP-0001 PATH-0001 CONTEXT-0001 CLIENT-0001 CMD-0001 PATH-0002 CONFIG-0001 SECRET-0001 SOURCE-0004; do
  grep -q "$rule" "$output/report.json"
done
if grep -q 'HARNESSSCOPE-CANARY' "$output/terminal.txt" "$output/report.json" "$output/report.html"; then
  printf '%s\n' 'synthetic canary leaked into demo output' >&2
  exit 1
fi

demo_hscope snapshot save demo-baseline "$demo/workspace" >"$output/saved.json"
(cd "$demo/workspace" && demo_hscope snapshot list) >"$output/snapshots.json"
demo_hscope snapshot diff demo-baseline "$demo/workspace" >"$output/unchanged.json"
grep -Eq '"changes":[[:space:]]*\[\]' "$output/unchanged.json"
# A new declaration guarantees normalized ADDED drift, independent of timestamps.
printf '\n[mcp_servers.new_demo_service]\ncommand = "harnessscope-demo-new-command"\n' >>"$demo/home/.codex/config.toml"
demo_hscope snapshot diff demo-baseline "$demo/workspace" >"$output/drift.json"
grep -q 'ADDED' "$output/drift.json"
demo_hscope export "$demo/workspace" --baseline demo-baseline --output "$output/diagnostic.zip" >"$output/export.txt"
mkdir "$output/bundle"
unzip -q "$output/diagnostic.zip" -d "$output/bundle"
for member in README.txt report.json report.html drift.json manifest.json; do
  test -s "$output/bundle/$member"
done
test "$(unzip -Z1 "$output/diagnostic.zip" | wc -l | tr -d ' ')" = 5
case "$(uname -s)" in
  Darwin) snapshot_root="$demo/home/Library/Application Support/HarnessScope/snapshots" ;;
  *) snapshot_root="$demo/home/.local/share/harnessscope/snapshots" ;;
esac
test -d "$snapshot_root"
# Check decoded JSON, HTML, terminal, snapshot and ZIP members, not just ZIP bytes.
if grep -RE 'HARNESSSCOPE-CANARY|/Users/|/home/[^/]+/' "$output/bundle" "$output/terminal.txt" "$output/report.json" "$output/report.html" "$output/drift.json" "$snapshot_root"; then
  printf '%s\n' 'private data leaked into demo output' >&2
  exit 1
fi
if grep -RF "$output" "$output/bundle" "$output/report.json" "$output/report.html" "$output/drift.json" "$snapshot_root"; then
  printf '%s\n' 'temporary machine path leaked into demo output' >&2
  exit 1
fi

cat "$output/terminal.txt"
printf 'Demo JSON: %s\nDemo HTML: %s\nDiagnostic ZIP: %s\n' "$output/report.json" "$output/report.html" "$output/diagnostic.zip"
