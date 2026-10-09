#!/usr/bin/env bash
# check: starlight-theme
# born: 2026-10-09
# failure: examples/deedbox-docs has an old copy of the Starlight theme
# rule: examples/deedbox-docs/app/starlight.css equals
# registry/starlight/starlight.css; copy the registry file to the example
set -euo pipefail
root="$(git rev-parse --show-toplevel)"
cd "$root"
if ! cmp -s registry/starlight/starlight.css examples/deedbox-docs/app/starlight.css; then
  echo "rule: examples/deedbox-docs/app/starlight.css differs from registry/starlight/starlight.css; copy the registry file to the example" >&2
  exit 1
fi
