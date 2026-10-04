#!/usr/bin/env bash
# check: tailwind-smoke
# born: 2026-10-04
# failure: the pinned Tailwind binary cannot be downloaded or run without node
# rule: REQ-STY-01: run the pinned Tailwind standalone binary with no node on PATH
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"
exec go test -tags tailwindreal ./internal/tailwind ./internal/gxstyles -run 'TestREQ_STY_01_RealBinary|TestREQ_STY_02_ClassListInCSS' -v -count=1
