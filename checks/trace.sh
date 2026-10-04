#!/usr/bin/env bash
# check: trace
# born: 2026-10-03
# failure: an ID was marked PASS with no covering test, or a test named an ID that does not exist
# rule: every ID named in a test exists in docs/SDD.md, and every PASS ID in docs/build/ledger.md has a covering test in the tree
set -euo pipefail
root="$(git rev-parse --show-toplevel)"
cd "$root"
if [ ! -f docs/SDD.md ] || [ ! -f docs/build/ledger.md ]; then
  echo "trace: skipped; docs/SDD.md and docs/build/ledger.md are local build state"
  exit 0
fi
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
fail=0

grep -oE 'REQ-[A-Z]+-[0-9]+|NFR-[0-9]+|SI-[0-9]+' docs/SDD.md | sort -u > "$tmp/sdd-ids"

find . -path ./.git -prune -o -path '*/node_modules' -prune -o -name '*_test.go' -type f -print |
  while IFS= read -r f; do
    grep -oE 'func +(Test|Benchmark|Example|Fuzz)[A-Za-z0-9_]*' "$f" || true
  done |
  sed 's/^func *//' | tr '_' '-' |
  grep -oE 'REQ-[A-Z]+-[0-9]+|NFR-[0-9]+|SI-[0-9]+' |
  sort -u > "$tmp/named-go" || true

find . -path ./.git -prune -o -path '*/node_modules' -prune -o -type f \( -name '*.ts' -o -name '*.tsx' \) -print |
  while IFS= read -r f; do
    grep -oE "(test|it)\([\"'\`][^\"'\`]*" "$f" || true
  done |
  grep -oE 'REQ-[A-Z]+-[0-9]+|NFR-[0-9]+|SI-[0-9]+' |
  sort -u > "$tmp/named-ts" || true

cat "$tmp/named-go" "$tmp/named-ts" | sort -u > "$tmp/named"

unknown="$(comm -13 "$tmp/sdd-ids" "$tmp/named" || true)"
if [ -n "$unknown" ]; then
  echo "rule: a test names an ID that is not in the SDD:" >&2
  echo "$unknown" >&2
  fail=1
fi

awk -F'|' '/^\|/ { gsub(/ /,"",$2); gsub(/ /,"",$5); if ($5=="PASS") print $2 }' docs/build/ledger.md | sort -u > "$tmp/pass-ids"
while IFS= read -r id; do
  [ -z "$id" ] && continue
  grep -qxF "$id" "$tmp/named" || { echo "rule: $id is PASS with no covering test" >&2; fail=1; }
done < "$tmp/pass-ids"

named="$(wc -l < "$tmp/named" | tr -d ' ')"
pass="$(wc -l < "$tmp/pass-ids" | tr -d ' ')"
open_no_test="$(comm -23 <(awk -F'|' '/^\|/ { gsub(/ /,"",$2); gsub(/ /,"",$5); if ($5=="OPEN") print $2 }' docs/build/ledger.md | sort -u) "$tmp/named" | grep -c . || true)"
echo "trace: $named IDs named in tests, $pass PASS, $open_no_test OPEN with no test"
exit "$fail"
