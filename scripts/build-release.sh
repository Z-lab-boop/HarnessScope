#!/bin/sh
set -eu

repository=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
version=${1:-v0.1.0-dev}
output=${2:-"$repository/dist"}
mkdir -p "$output"

for target in darwin-amd64 darwin-arm64 linux-amd64 linux-arm64; do
  target_os=${target%-*}
  target_arch=${target#*-}
  name="harnessscope_${version}_${target_os}_${target_arch}"
  staging=$(mktemp -d "${TMPDIR:-/tmp}/harnessscope-release.XXXXXX")
  (
    cd "$repository"
    CGO_ENABLED=0 GOOS="$target_os" GOARCH="$target_arch" go build -trimpath -ldflags="-s -w" -o "$staging/hscope" ./cmd/hscope
  )
  cp "$repository/LICENSE" "$repository/README.md" "$repository/README.zh-CN.md" "$repository/THIRD_PARTY_NOTICES.md" "$staging/"
  tar -C "$staging" -czf "$output/$name.tar.gz" hscope LICENSE README.md README.zh-CN.md THIRD_PARTY_NOTICES.md
  rm -rf "$staging"
done

(
  cd "$output"
  shasum -a 256 harnessscope_*.tar.gz >SHA256SUMS
)
