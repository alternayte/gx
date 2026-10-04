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
    cd tests/e2e && bun install --frozen-lockfile
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

# The 100-page export budget (NFR-11).
bench-export:
    go test ./internal/exporter -run TestNFR_11 -v -count=1 -timeout 300s

bench-dev:
    go test ./internal/devserver -run TestNFR_02 -v

# The Deedbox docs parity checklist (REQ-CNT-14).
parity-docs:
    cd tests/e2e && bun test deedbox.spec.ts

# The LSP latency budgets (NFR-06).
bench-lsp:
    go test ./internal/lsp -run TestNFR_06 -v

# The VS Code extension smoke test (REQ-TLS-05). Needs node.
vscode-smoke:
    "scripts/vscode-smoke.sh"

# The headless Neovim smoke test (REQ-TLS-06). Needs nvim 0.11+.
nvim-smoke:
    "scripts/nvim-smoke.sh"

# The Delve breakpoint test (REQ-TLS-08). Needs dlv.
delve-smoke:
    "scripts/delve-smoke.sh"

# The real Tailwind download test (REQ-STY-01). Needs network.
tailwind-smoke:
    "scripts/tailwind-smoke.sh"

evidence:
    go run ./internal/build/evidence --write

evidence-check:
    go run ./internal/build/evidence --check
