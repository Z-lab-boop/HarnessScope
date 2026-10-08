#!/bin/sh
set -eu
repository=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
output=$(mktemp -d "${TMPDIR:-/tmp}/harnessscope-dashboard-demo.XXXXXX")
output=$(CDPATH= cd -- "$output" && pwd -P)
binary="$output/hscope"
child=
cleanup() {
  if [ -n "$child" ]; then
    kill -TERM "$child" 2>/dev/null || true
    wait "$child" 2>/dev/null || true
  fi
  rm -rf "$output"
}
trap cleanup EXIT
trap 'exit 0' INT TERM
. "$repository/demo/prepare.sh"
prepare_demo
(cd "$repository" && go build -trimpath -o "$binary" ./cmd/hscope)
# Run the binary directly so the trapped PID is the server, not a shell wrapper.
HOME="$demo/home" XDG_DATA_HOME="$demo/home/.local/share" PATH="$demo/home/bin:/usr/bin:/bin" \
  "$binary" serve "$demo/workspace" --port 0 &
child=$!
wait "$child"
