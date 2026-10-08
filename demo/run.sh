#!/bin/sh
set -eu

repository=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
demo="$repository/demo/conflicted-workspace"
output=$(mktemp -d "${TMPDIR:-/tmp}/harnessscope-demo.XXXXXX")
binary="$output/hscope"

(
  cd "$repository"
  go build -trimpath -o "$binary" ./cmd/hscope
)

HOME="$demo/home" PATH="$demo/bin:$PATH" "$binary" scan "$demo/workspace" \
  --json "$output/report.json" --html "$output/report.html" >"$output/terminal.txt"

for rule in MCP-0001 PATH-0001 CONTEXT-0001; do
  grep -q "$rule" "$output/report.json"
done
if grep -q 'HARNESSSCOPE-CANARY' "$output/terminal.txt" "$output/report.json" "$output/report.html"; then
  printf '%s\n' 'synthetic canary leaked into demo output' >&2
  exit 1
fi

cat "$output/terminal.txt"
printf 'Demo JSON: %s\nDemo HTML: %s\n' "$output/report.json" "$output/report.html"
