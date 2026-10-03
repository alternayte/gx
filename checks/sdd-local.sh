#!/usr/bin/env bash
# check: sdd-local
# born: 2026-10-03
# failure: the spec and the build state are private, and a public repo must never receive them
# rule: docs/SDD.md and docs/build/ are ignored by git and not tracked
set -euo pipefail

root="$(git rev-parse --show-toplevel)"
cd "$root"
fail=0
for path in docs/SDD.md docs/build/; do
  if [ -n "$(git ls-files -- "$path")" ]; then
    echo "rule: $path is tracked; run git rm -r --cached $path" >&2
    fail=1
  fi
  if ! git check-ignore -q "$path"; then
    echo "rule: $path is not ignored; add it to .gitignore" >&2
    fail=1
  fi
done
exit "$fail"
