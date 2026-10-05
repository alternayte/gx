#!/usr/bin/env bash
# Runs one browser test file and records the run for `just evidence`: a JUnit
# report of every test, and a stamp of the commit and tree state the run saw.
# The exit code is the one of bun test.
set -uo pipefail
cd "$(dirname "$0")"
f="$1"
name="$(basename "$f")"
mkdir -p .results
rm -f ".results/$name.xml" ".results/$name.json"

# A run counts as clean only when the tree matches HEAD before and after it.
tree() {
  git rev-parse HEAD 2>/dev/null || return 1
  git status --porcelain 2>/dev/null
}
before="$(tree)" || before=""
bun test --timeout=300000 --reporter=junit --reporter-outfile=".results/$name.xml" "$f"
rc=$?
after="$(tree)" || after=""
commit="${after%%$'\n'*}"
clean=false
if [ -n "$commit" ] && [ "$before" = "$commit" ] && [ "$after" = "$commit" ]; then
  clean=true
fi
printf '{"commit":"%s","clean":%s}\n' "$commit" "$clean" > ".results/$name.json"
exit "$rc"
