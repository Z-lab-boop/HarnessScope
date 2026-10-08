#!/bin/sh
set -eu
repository=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
test "$#" = 1 || { printf 'usage: %s archive.tar.gz\n' "$0" >&2; exit 2; }
archive=$1
check_root=$(mktemp -d "${TMPDIR:-/tmp}/hscope-archive-check.XXXXXX")
trap 'rm -rf "$check_root"' EXIT
trap 'exit 1' INT TERM
LC_ALL=C sort "$repository/scripts/release-files.txt" >"$check_root/expected"
tar -tzf "$archive" | LC_ALL=C sort >"$check_root/actual"
diff -u "$check_root/expected" "$check_root/actual"
# An allowlisted name must still be a regular file, never a link or device.
tar -tvzf "$archive" >"$check_root/types"
if grep -v '^-' "$check_root/types" >/dev/null; then
  printf '%s\n' 'release archive contains a nonregular member' >&2; exit 1
fi
mkdir "$check_root/extracted"
tar -C "$check_root/extracted" -xzf "$archive"
while IFS= read -r member; do
  file="$check_root/extracted/$member"
  test -f "$file" && test ! -L "$file"
  if grep -Fq "$repository" "$file"; then
    printf 'build-machine root found in %s\n' "$member" >&2; exit 1
  fi
  if [ "$member" != hscope ] && grep -Eq '(^|[[:space:]"`=(])/(Users|home)/[^/[:space:]]+|/private/var/folders/|/var/folders/|HARNESSSCOPE-CANARY' "$file"; then
    printf 'private path or canary in public archive member: %s\n' "$member" >&2; exit 1
  fi
done <"$repository/scripts/release-files.txt"
printf '%s\n' 'Release archive exact public allowlist and extracted privacy PASS'
