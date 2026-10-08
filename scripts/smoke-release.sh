#!/bin/sh
set -eu

if [ "$#" -lt 1 ] || [ "$#" -gt 2 ]; then
  printf 'usage: %s /path/to/hscope [expected-version]\n' "$0" >&2
  exit 2
fi

case "$1" in
  /*) binary=$1 ;;
  *) binary=$(CDPATH= cd -- "$(dirname -- "$1")" && pwd)/$(basename -- "$1") ;;
esac
expected_version=${2:-0.2.0-dev}
expected_version=${expected_version#v}
test "$("$binary" --version)" = "hscope version $expected_version"
smoke_root=$(mktemp -d "${TMPDIR:-/tmp}/harnessscope-smoke.XXXXXX")
smoke_root=$(CDPATH= cd -- "$smoke_root" && pwd -P)
server_pid=
cleanup() {
  if [ -n "$server_pid" ]; then
    kill -TERM "$server_pid" 2>/dev/null || true
    wait "$server_pid" 2>/dev/null || true
  fi
  rm -rf "$smoke_root"
}
trap cleanup EXIT
trap 'exit 1' INT TERM
for managed in /etc/codex '/Library/Application Support/ClaudeCode/managed-settings.json' /etc/claude-code/managed-settings.json; do
  test ! -e "$managed" || { printf '%s\n' 'Smoke test requires a host without managed agent configuration' >&2; exit 1; }
done

mkdir -p "$smoke_root/home/.cursor" "$smoke_root/home/bin" "$smoke_root/workspace"
printf '%s\n' '#!/bin/sh' "printf '%s\\n' 'cursor smoke-preview'" >"$smoke_root/home/bin/cursor"
chmod 700 "$smoke_root/home/bin/cursor"
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
export HOME="$smoke_root/home" XDG_DATA_HOME="$smoke_root/home/.local/share" PATH="$smoke_root/home/bin:/usr/bin:/bin"
"$binary" scan "$smoke_root/workspace" --client cursor --no-report >"$smoke_root/output.txt"
grep -q 'PREVIEW' "$smoke_root/output.txt"
grep -q 'PATH-0001' "$smoke_root/output.txt"
if grep -q 'HARNESSSCOPE-CANARY' "$smoke_root/output.txt"; then
  printf '%s\n' 'canary leaked during release smoke test' >&2
  exit 1
fi
"$binary" snapshot save release-smoke "$smoke_root/workspace" >"$smoke_root/saved.json"
"$binary" snapshot diff release-smoke "$smoke_root/workspace" >"$smoke_root/drift.json"
grep -Eq '"changes":[[:space:]]*\[\]' "$smoke_root/drift.json"
"$binary" export "$smoke_root/workspace" --baseline release-smoke --output "$smoke_root/diagnostic.zip" >/dev/null
unzip -t "$smoke_root/diagnostic.zip" >/dev/null
mkdir "$smoke_root/bundle"
unzip -q "$smoke_root/diagnostic.zip" -d "$smoke_root/bundle"
grep -Fq "\"tool_version\": \"$expected_version\"" "$smoke_root/bundle/manifest.json"
"$binary" serve "$smoke_root/workspace" --client cursor --port 0 >"$smoke_root/launch.txt" 2>"$smoke_root/server.log" &
server_pid=$!
attempt=0
while [ ! -s "$smoke_root/launch.txt" ]; do
  kill -0 "$server_pid"
  attempt=$((attempt + 1))
  test "$attempt" -lt 100
  sleep 0.1
done
test "$(wc -l <"$smoke_root/launch.txt" | tr -d ' ')" = 1
launch=$(cat "$smoke_root/launch.txt")
case "$launch" in http://127.0.0.1:*'/#token='*) ;; *) exit 1 ;; esac
base=${launch%%/#token=*}
token=${launch#*#token=}
for route in state explain compare backups snapshots fixes/plan fixes/apply rollback rescan drift export; do
  code=$(curl --noproxy '*' --silent --output /dev/null --write-out '%{http_code}' "$base/api/v1/$route")
  test "$code" = 401
done
curl --noproxy '*' --fail --silent -H "X-HarnessScope-Token: $token" "$base/api/v1/state" >"$smoke_root/state.json"
grep -q '"revision"' "$smoke_root/state.json"
revision=$(sed -n 's/.*"revision":[ ]*\([0-9][0-9]*\).*/\1/p' "$smoke_root/state.json")
curl --noproxy '*' --fail --silent -H "X-HarnessScope-Token: $token" -H "Origin: $base" -H 'Content-Type: application/json' \
  --data "{\"revision\":$revision}" "$base/api/v1/export" >"$smoke_root/browser.zip"
unzip -p "$smoke_root/browser.zip" manifest.json >"$smoke_root/browser-manifest.json"
grep -Fq "\"tool_version\": \"$expected_version\"" "$smoke_root/browser-manifest.json"
curl --noproxy '*' --fail --silent "$base/" >"$smoke_root/dashboard.html"
grep -q 'dashboard.js' "$smoke_root/dashboard.html"
if grep -RE 'HARNESSSCOPE-CANARY|/Users/|/home/[^/]+/' "$smoke_root/bundle" "$smoke_root/state.json" "$smoke_root/server.log"; then exit 1; fi
if grep -RF "$smoke_root" "$smoke_root/bundle" "$smoke_root/state.json" "$smoke_root/server.log"; then exit 1; fi
if grep -F "$token" "$smoke_root/state.json" "$smoke_root/server.log"; then exit 1; fi
kill -INT "$server_pid"
wait "$server_pid"
server_pid=
if curl --noproxy '*' --silent --max-time 2 "$base/" >/dev/null; then exit 1; fi
printf '%s\n' 'Release smoke PASS: CLI, snapshot, drift, ZIP, authenticated loopback dashboard, clean SIGINT'
