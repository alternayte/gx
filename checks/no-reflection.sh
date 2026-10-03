#!/usr/bin/env bash
# check: no-reflection
# born: 2026-10-03
# failure: the render path used reflection and every render paid for it
# rule: no Go file in the repo imports reflect
set -euo pipefail
root="$(git rev-parse --show-toplevel)"
cd "$root"
hits="$(find . -path ./.git -prune -o -type f \( -name '*.go' -o -name '*_gx.go' \) -print | grep -v '_test.go' | xargs grep -l '"reflect"' 2>/dev/null || true)"
if [ -n "$hits" ]; then
  echo "rule: reflection is not allowed on the render path:" >&2
  echo "$hits" >&2
  exit 1
fi
exit 0
