#!/usr/bin/env bash
# check: bench-render
# born: 2026-10-03
# failure: the compiled render path fell behind templ
# rule: gx render is at most 1.5x templ render on the same page set
set -euo pipefail
root="$(git rev-parse --show-toplevel)"
cd "$root"
out="$(go test -run '^$' -bench 'BenchmarkRender' -benchmem -count=3 ./internal/bench/ 2>&1)"
gx="$(printf '%s\n' "$out" | awk '/^BenchmarkRender-/ { if ($3+0 > 0 && (min == "" || $3+0 < min)) min = $3+0 } END { print min }')"
templ="$(printf '%s\n' "$out" | awk '/^BenchmarkRenderTempl-/ { if ($3+0 > 0 && (min == "" || $3+0 < min)) min = $3+0 } END { print min }')"
if [ -z "$gx" ] || [ -z "$templ" ]; then
  echo "rule: benchmark output not understood" >&2
  printf '%s\n' "$out" >&2
  exit 1
fi
awk -v gx="$gx" -v templ="$templ" 'BEGIN {
  ratio = gx / templ
  printf "bench-render: gx %d ns/op, templ %d ns/op, ratio %.2f\n", gx, templ, ratio
  if (ratio > 1.5) {
    printf "rule: gx render is %.2fx templ, budget is 1.5x\n", ratio > "/dev/stderr"
    exit 1
  }
}'
