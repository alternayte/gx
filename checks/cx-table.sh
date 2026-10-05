#!/usr/bin/env bash
# check: cx-table
# born: 2026-10-05
# failure: gx.Cx was a hand-written subset of tailwind-merge, and nothing tied it to the pinned release
# rule: cxtable.go equals the output of tools/twmerge/gen.ts for the tailwind-merge release in tools/shadcn-ref/bun.lock
set -euo pipefail
root="$(git rev-parse --show-toplevel)"
cd "$root"
(cd tools/shadcn-ref && bun install --frozen-lockfile >/dev/null 2>&1)
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
bun run tools/twmerge/gen.ts tools/shadcn-ref/node_modules/tailwind-merge/src "$tmp/cxtable.go" >/dev/null
gofmt -w "$tmp/cxtable.go"
if ! diff -q "$tmp/cxtable.go" cxtable.go >/dev/null; then
  echo "rule: cxtable.go is stale for the pinned tailwind-merge; run just cx-table" >&2
  exit 1
fi
