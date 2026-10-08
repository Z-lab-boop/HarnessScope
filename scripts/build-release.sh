#!/bin/sh
set -eu

repository=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
version=${1:-v0.2.0-dev}
output=${2:-"$repository/dist"}
case "$version" in *[!A-Za-z0-9._-]*|'') printf '%s\n' 'invalid release version' >&2; exit 2 ;; esac
mkdir -p "$output"
output=$(CDPATH= cd -- "$output" && pwd)
if [ -n "$(ls -A "$output")" ]; then
  printf '%s\n' 'release output directory must be empty' >&2
  exit 2
fi
staging=
trap 'if [ -n "$staging" ]; then rm -rf "$staging"; fi' EXIT
trap 'exit 1' INT TERM
"$repository/scripts/check-third-party-notices.sh"

for target in darwin-amd64 darwin-arm64 linux-amd64 linux-arm64; do
  target_os=${target%-*}
  target_arch=${target#*-}
  name="harnessscope_${version}_${target_os}_${target_arch}"
  staging=$(mktemp -d "${TMPDIR:-/tmp}/harnessscope-release.XXXXXX")
  (
    cd "$repository"
    CGO_ENABLED=0 GOOS="$target_os" GOARCH="$target_arch" go build -trimpath -buildvcs=false -ldflags="-s -w" -o "$staging/hscope" ./cmd/hscope
  )
  cp "$repository/LICENSE" "$repository/README.md" "$repository/README.zh-CN.md" "$repository/THIRD_PARTY_NOTICES.md" "$repository/SECURITY.md" "$staging/"
  cp -R "$repository/docs" "$staging/docs"
  tar -C "$staging" -czf "$output/$name.tar.gz" hscope LICENSE README.md README.zh-CN.md THIRD_PARTY_NOTICES.md SECURITY.md docs
  rm -rf "$staging"
  staging=
done

(
  cd "$output"
  shasum -a 256 harnessscope_*.tar.gz >SHA256SUMS
  shasum -a 256 -c SHA256SUMS
)
