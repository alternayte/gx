#!/usr/bin/env bash
# check: e2e
# born: 2026-10-03
# failure: a browser test fails in the example app
# rule: bun test passes; the known playwright-core previewUpdated teardown
# race is ignored only when every test passed
set -uo pipefail
cd "$(dirname "$0")"
log="$(bun test 2>&1)"
code=$?
printf '%s\n' "$log"
if [ "$code" -eq 0 ]; then
  exit 0
fi
if grep -q 'Cannot find object to "previewUpdated"' <<<"$log" && grep -qE '[0-9]+ fail' <<<"$log" && grep -q ' 0 fail' <<<"$log"; then
  echo "e2e: ignored the playwright-core previewUpdated teardown race"
  exit 0
fi
exit "$code"
