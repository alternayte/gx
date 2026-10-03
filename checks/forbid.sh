#!/usr/bin/env bash
# check: forbid
# born: 2026-10-03
# failure: a stub marker or a skipped test shipped in the repo
# rule: no word from scripts/forbid-words.txt appears in source, and no test skips outside tests/quarantine/
set -euo pipefail
root="$(git rev-parse --show-toplevel)"
cd "$root"
fail=0

files="$(find . -path ./.git -prune -o -path '*/node_modules' -prune -o -type f \( -name '*.go' -o -name '*.ts' -o -name '*.tsx' -o -name '*.gx' -o -name '*.js' \) -print)"
while IFS= read -r word; do
  case "$word" in ''|\#*) continue ;; esac
  hits="$(printf '%s\n' "$files" | xargs grep -Fn -- "$word" 2>/dev/null || true)"
  if [ -n "$hits" ]; then
    echo "rule: stub marker '$word':" >&2
    echo "$hits" >&2
    fail=1
  fi
done < scripts/forbid-words.txt

skips="$(find . -path ./.git -prune -o -path '*/node_modules' -prune -o -path ./tests/quarantine -prune -o -type f -name '*_test.go' -print | xargs grep -En 't\.Skip|testing\.Skip' 2>/dev/null || true)"
if [ -n "$skips" ]; then
  echo "rule: a test skips outside tests/quarantine/:" >&2
  echo "$skips" >&2
  fail=1
fi

if grep -q '^status: DONE' docs/build/ledger.md 2>/dev/null; then
  if [ -d tests/quarantine ] && [ -n "$(find tests/quarantine -type f -print -quit 2>/dev/null)" ]; then
    echo "rule: tests/quarantine must be empty at DONE" >&2
    fail=1
  fi
fi
exit "$fail"
