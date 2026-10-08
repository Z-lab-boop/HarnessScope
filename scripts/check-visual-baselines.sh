#!/bin/sh
set -eu
repository=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
available=true
for file in web/tests/__snapshots__/report-linux.png web/dashboard/tests/__snapshots__/dashboard-linux.png web/dashboard/tests/__snapshots__/graph-linux.png; do
  test -s "$repository/$file" || available=false
done
if [ -n "${GITHUB_OUTPUT:-}" ]; then printf 'available=%s\n' "$available" >>"$GITHUB_OUTPUT"; fi
if [ "$available" = true ]; then
  printf '%s\n' 'Reviewed Linux artifact set present; comparison is required.'
  exit 0
fi
message='OPEN: reviewed Linux report, Overview and Graph baselines are absent; see docs/visual-baselines.md'
if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then printf '%s\n' "$message" >>"$GITHUB_STEP_SUMMARY"; fi
if [ "${HARNESSSCOPE_STRICT_RELEASE:-false}" = true ]; then
  printf '::error::%s\n' "$message"; exit 1
fi
printf '::warning::%s (non-blocking functional CI)\n' "$message"
