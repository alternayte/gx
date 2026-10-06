#!/usr/bin/env bash
# sync-shop reinstalls the current registry into examples/shop (D-135). The
# shop never edits its copies, so the script removes each installed item and
# adds it again; it needs no base snapshot and works in a fresh clone. The
# shop gallery, the e2e suites and the parity suite render these copies.
set -euo pipefail
root="$(git rev-parse --show-toplevel)"
cd "$root"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
go build -o "$tmp/gx" ./cmd/gx
"$tmp/gx" registry build --out "$tmp/registry" registry >/dev/null
names=()
for dir in registry/*/; do
  name="$(basename "$dir")"
  [ -f "$dir/gx-item.json" ] || continue
  # The docs kit has its own reference app (D-136).
  case "$name" in docs | docs-shell) continue ;; esac
  names+=("$name")
  rm -rf "examples/shop/ui/$name" "examples/shop/.gx/base/$name@"*
done
# gx.lock stays: it also holds the pins of gx pin and gx wc pin. gx add
# writes the entry of each item again.
for name in "${names[@]}"; do
  "$tmp/gx" add --registry "$tmp/registry" "$name" examples/shop >/dev/null
done
# build also writes the stylesheet package with the new classes.
"$tmp/gx" build -o "$tmp/shop" examples/shop
