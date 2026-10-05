#!/usr/bin/env bash
# check: runtime
# born: 2026-10-03
# failure: the committed browser runtime does not match runtime/js/gx.ts
# rule: `bun build` of runtime/js/gx.ts equals the committed runtime/js/gx.js
set -euo pipefail
root="$(git rev-parse --show-toplevel)"
cd "$root"
if ! command -v bun >/dev/null 2>&1; then
  echo "runtime: bun is not on PATH; the browser runtime cannot be rebuilt" >&2
  exit 1
fi
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
bun build runtime/js/gx.ts --outfile "$tmp/gx.js" --target browser --minify >/dev/null
if ! cmp -s "$tmp/gx.js" runtime/js/gx.js; then
  echo "rule: runtime/js/gx.js is stale; run just runtime" >&2
  diff -u runtime/js/gx.js "$tmp/gx.js" >&2 || true
  exit 1
fi
bun build runtime/js/behavior.ts --outfile "$tmp/behavior.js" --target browser --minify >/dev/null
if ! cmp -s "$tmp/behavior.js" runtime/js/behavior.js; then
  echo "rule: runtime/js/behavior.js is stale; run just runtime" >&2
  diff -u runtime/js/behavior.js "$tmp/behavior.js" >&2 || true
  exit 1
fi
bun build runtime/js/tabs.ts --outfile "$tmp/tabs.js" --target browser --minify >/dev/null
if ! cmp -s "$tmp/tabs.js" runtime/js/tabs.js; then
  echo "rule: runtime/js/tabs.js is stale; run just runtime" >&2
  diff -u runtime/js/tabs.js "$tmp/tabs.js" >&2 || true
  exit 1
fi
bun build runtime/js/toast.ts --outfile "$tmp/toast.js" --target browser --minify >/dev/null
if ! cmp -s "$tmp/toast.js" runtime/js/toast.js; then
  echo "rule: runtime/js/toast.js is stale; run just runtime" >&2
  diff -u runtime/js/toast.js "$tmp/toast.js" >&2 || true
  exit 1
fi
bun build runtime/js/overlay.ts --outfile "$tmp/overlay.js" --target browser --minify >/dev/null
if ! cmp -s "$tmp/overlay.js" runtime/js/overlay.js; then
  echo "rule: runtime/js/overlay.js is stale; run just runtime" >&2
  diff -u runtime/js/overlay.js "$tmp/overlay.js" >&2 || true
  exit 1
fi
bun build runtime/js/theme.ts --outfile "$tmp/theme.js" --target browser --minify >/dev/null
if ! cmp -s "$tmp/theme.js" runtime/js/theme.js; then
  echo "rule: runtime/js/theme.js is stale; run just runtime" >&2
  diff -u runtime/js/theme.js "$tmp/theme.js" >&2 || true
  exit 1
fi
bun build runtime/js/dev.ts --outfile "$tmp/devclient.js" --target browser --minify >/dev/null
if ! cmp -s "$tmp/devclient.js" internal/devserver/devclient.js; then
  echo "rule: internal/devserver/devclient.js is stale; run just runtime" >&2
  diff -u internal/devserver/devclient.js "$tmp/devclient.js" >&2 || true
  exit 1
fi
