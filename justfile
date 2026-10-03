# Gx build gate.
set shell := ["bash", "-euo", "pipefail", "-c"]

# The gate. Run this before every commit.
verify: build vet test checks

# The same gate under the global agent-instruction name.
check: verify

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
    go test -run '^$' -bench BenchmarkRender -benchmem .

evidence:
    go run ./internal/build/evidence --write

evidence-check:
    go run ./internal/build/evidence --check
