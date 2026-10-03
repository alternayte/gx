# Gx build gate.
set shell := ["bash", "-euo", "pipefail", "-c"]

# The gate. Run this before every commit.
verify: build vet runtime test checks e2e

# The same gate under the global agent-instruction name.
check: verify

# Build the browser runtime from its TypeScript source.
runtime:
    bun build runtime/js/gx.ts --outfile runtime/js/gx.js --target browser --minify
    bun build runtime/js/dev.ts --outfile internal/devserver/devclient.js --target browser --minify

# Drive the real app in a real browser. The repo's own gate only (G3).
e2e:
    "tests/e2e/run.sh"

build:
    rm -rf .gx-build; mkdir -p .gx-build; trap 'rm -rf .gx-build' EXIT; go build -o .gx-build ./...

vet:
    go vet ./...

test:
    go test ./...

checks:
    for f in checks/*.sh; do if [ -x "$f" ]; then "$f"; fi; done

trace:
    "checks/trace.sh"

forbid:
    "checks/forbid.sh"

bench-render:
    "scripts/bench-render.sh"

bench-build:
    "scripts/bench-build.sh"

bench-dev:
    go test ./internal/devserver -run TestNFR_02 -v

evidence:
    go run ./internal/build/evidence --write

evidence-check:
    go run ./internal/build/evidence --check
