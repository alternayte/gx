#!/usr/bin/env bash
# check: no-reflection
# born: 2026-10-03
# failure: the render path used reflection and every render paid for it
# rule: the runtime package gx (root non-test .go files) does not import reflect
set -euo pipefail
root="$(git rev-parse --show-toplevel)"
cd "$root"
hits="$(find . -maxdepth 1 -type f -name '*.go' ! -name '*_test.go' -print | xargs grep -l '"reflect"' 2>/dev/null || true)"
if [ -n "$hits" ]; then
  echo "rule: reflection is not allowed on the render path:" >&2
  echo "$hits" >&2
  exit 1
fi
exit 0
