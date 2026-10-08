#!/bin/sh
set -eu

repository=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
version=${1:-v0.2.0-dev}
output=${2:-"$repository/dist"}
normalized_version=${version#v}
case "$normalized_version" in *[!A-Za-z0-9.+-]*|'') printf '%s\n' 'invalid release version' >&2; exit 2 ;; esac
if ! printf '%s\n' "$normalized_version" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+(-[A-Za-z0-9.-]+)?(\+[A-Za-z0-9.-]+)?$' || [ "${#normalized_version}" -gt 128 ]; then
  printf '%s\n' 'invalid release version' >&2; exit 2
fi
targets=${3:-'darwin-amd64 darwin-arm64 linux-amd64 linux-arm64'}
for target in $targets; do
  case "$target" in darwin-amd64|darwin-arm64|linux-amd64|linux-arm64) ;; *) printf '%s\n' 'invalid release target' >&2; exit 2 ;; esac
done
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

for target in $targets; do
  target_os=${target%-*}
  target_arch=${target#*-}
  name="harnessscope_${version}_${target_os}_${target_arch}"
  staging=$(mktemp -d "${TMPDIR:-/tmp}/harnessscope-release.XXXXXX")
  (
    cd "$repository"
    CGO_ENABLED=0 GOOS="$target_os" GOARCH="$target_arch" go build -trimpath -buildvcs=false -ldflags="-s -w -X github.com/Z-lab-boop/harnessscope/internal/buildinfo.Version=$normalized_version" -o "$staging/hscope" ./cmd/hscope
  )
  while IFS= read -r member; do
    [ "$member" = hscope ] && continue
    mkdir -p "$staging/$(dirname -- "$member")"
    cp "$repository/$member" "$staging/$member"
  done <"$repository/scripts/release-files.txt"
  tar -C "$staging" -czf "$output/$name.tar.gz" -T "$repository/scripts/release-files.txt"
  "$repository/scripts/check-release-archive.sh" "$output/$name.tar.gz"
  rm -rf "$staging"
  staging=
done

(
  cd "$output"
  shasum -a 256 harnessscope_*.tar.gz >SHA256SUMS
  shasum -a 256 -c SHA256SUMS
)
