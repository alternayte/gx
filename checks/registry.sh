#!/usr/bin/env bash
# check: registry
# born: 2026-10-05
# failure: the published registry files do not match the registry sources
# rule: `gx registry build` of registry/ equals the committed
# registry/index.json and registry/items/; run just registry
set -euo pipefail
root="$(git rev-parse --show-toplevel)"
cd "$root"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
go run ./cmd/gx registry build --out "$tmp" registry >/dev/null
if ! cmp -s "$tmp/index.json" registry/index.json; then
  echo "rule: registry/index.json is stale; run just registry" >&2
  exit 1
fi
if ! diff -rq "$tmp/items" registry/items >&2; then
  echo "rule: registry/items is stale; run just registry" >&2
  exit 1
fi
