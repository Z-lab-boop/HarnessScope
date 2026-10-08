#!/bin/sh
set -eu

repository=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
notices="$repository/THIRD_PARTY_NOTICES.md"

go_hash=$(shasum -a 256 "$repository/go.sum" | awk '{print $1}')
npm_hash=$(shasum -a 256 "$repository/web/package-lock.json" | awk '{print $1}')

grep -q "$go_hash" "$notices" || {
  printf '%s\n' 'go.sum changed; audit dependencies and update THIRD_PARTY_NOTICES.md' >&2
  exit 1
}
grep -q "$npm_hash" "$notices" || {
  printf '%s\n' 'web/package-lock.json changed; audit dependencies and update THIRD_PARTY_NOTICES.md' >&2
  exit 1
}
