#!/usr/bin/env bash
# check: bench-build
# born: 2026-10-03
# failure: a build budget of NFR-05 was missed
# rule: cold gx build of the example app under 10s and incremental compile of
# one .gx file under 50ms
set -uo pipefail
root="$(git rev-parse --show-toplevel)"
cd "$root"
go test ./internal/compiler -run '^$' -bench 'BenchmarkNFR_05' -benchtime 1x -v
