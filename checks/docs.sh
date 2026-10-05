#!/usr/bin/env bash
# check: docs
# born: 2026-10-05
# failure: a registry item changed and the component pages of the docs site still showed the old item
# rule: `go run ./internal/docsgen --check` finds no stale file under docs/, and `gx check docs` passes
set -euo pipefail
root="$(git rev-parse --show-toplevel)"
cd "$root"
if ! go run ./internal/docsgen --check; then
  echo "rule: the generated docs pages are stale; run just docs-gen" >&2
  exit 1
fi
if ! go run ./cmd/gx check docs; then
  echo "rule: the generated Go of the docs app is stale or its content does not check; run just docs-gen" >&2
  exit 1
fi
