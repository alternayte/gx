#!/usr/bin/env bash
# check: lint
# born: 2026-10-05
# failure: gx lint printed 207 GX5003 findings on the example app, because the analyzer flagged every prop and gx.Enum lookup passed to gx.Cx, and nothing ran gx lint on an app with registry items
# rule: `gx lint` passes on examples/shop, which holds every tier 1 and tier 2 registry item
set -euo pipefail
root="$(git rev-parse --show-toplevel)"
cd "$root"
if ! go run ./cmd/gx lint examples/shop; then
  echo "rule: gx lint must pass on examples/shop; fix the finding or the analyzer" >&2
  exit 1
fi
