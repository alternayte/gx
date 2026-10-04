#!/usr/bin/env bash
# check: e2e
# born: 2026-10-03
# failure: a browser test fails in the example app
# rule: every spec file passes in its own process; the known playwright-core
# previewUpdated teardown race is ignored only when no test failed
set -uo pipefail
cd "$(dirname "$0")"
log=""
code=0
for f in *.spec.ts; do
  out="$(bun test --timeout=300000 "$f" 2>&1)"
  rc=$?
  printf '%s\n' "$out"
  log+="$out"$'\n'
  if [ "$rc" -ne 0 ]; then
    code="$rc"
  fi
done
if [ "$code" -eq 0 ]; then
  exit 0
fi
if ! grep -qE '\(fail\)' <<<"$log" && grep -q 'Cannot find object to "previewUpdated"' <<<"$log"; then
  echo "e2e: ignored the playwright-core previewUpdated teardown race"
  exit 0
fi
exit "$code"
