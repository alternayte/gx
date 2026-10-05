# Gx build gate.
set shell := ["bash", "-euo", "pipefail", "-c"]

# The gate. Run this before every commit.
verify: build vet runtime test checks e2e parity fallback

# The same gate under the global agent-instruction name.
check: verify

# Build the browser runtime from its TypeScript source.
runtime:
    bun build runtime/js/gx.ts --outfile runtime/js/gx.js --target browser --minify
    bun build runtime/js/behavior.ts --outfile runtime/js/behavior.js --target browser --minify
    bun build runtime/js/tabs.ts --outfile runtime/js/tabs.js --target browser --minify
    bun build runtime/js/toast.ts --outfile runtime/js/toast.js --target browser --minify
    bun build runtime/js/overlay.ts --outfile runtime/js/overlay.js --target browser --minify
    bun build runtime/js/theme.ts --outfile runtime/js/theme.js --target browser --minify
    bun build runtime/js/dev.ts --outfile internal/devserver/devclient.js --target browser --minify

# Drive the real app in a real browser. The repo's own gate only (G3).
e2e:
    cd tests/e2e && bun install --frozen-lockfile
    bash tests/e2e/run.sh

# Visual parity against the dev-only shadcn reference (REQ-REG-08).
parity:
    cd tools/shadcn-ref && bun install --frozen-lockfile
    go run ./internal/refcss -in tools/shadcn-ref/src/theme.css -out tools/shadcn-ref/public/ref.css
    cd tests/e2e && bun install --frozen-lockfile
    bash tests/e2e/record.sh ./parity.browsers.ts

build:
    rm -rf .gx-build; mkdir -p .gx-build; trap 'rm -rf .gx-build' EXIT; go build -o .gx-build ./...

vet:
    go vet ./...

# The time budgets (NFR-02, NFR-06, NFR-11, REQ-DEV-11) are timings on a quiet
# machine, so their packages run first, one at a time; the full run then takes
# their results from the cache.
test:
    go test -p 1 ./internal/devserver ./internal/lsp ./internal/exporter
    go test ./...
    cd docs && go test ./...

# Write the component pages of the docs site from registry/, then the
# generated Go and the stylesheet of the docs app (REQ-DOC-02).
docs-gen:
    go run ./internal/docsgen
    tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT; go run ./cmd/gx build -main . -o "$tmp/docs" docs

# Rebuild the published files of the official registry: index.json and
# items/. `gx add` reads them when the registry directory is the source.
registry:
    tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT; go run ./cmd/gx registry build --out "$tmp" registry && cp "$tmp/index.json" registry/index.json && rm -rf registry/items && cp -R "$tmp/items" registry/items

# Run the docs site with rebuild and reload.
docs-dev:
    cd docs && go run ../cmd/gx dev -main .

# Export the docs site to docs/dist.
docs-export:
    go run ./cmd/gx export -main . --out docs/dist docs

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

# Rebuild the class group table of gx.Cx from the pinned tailwind-merge
# (REQ-STY-04). Bun reads the tailwind-merge source in tools/shadcn-ref.
cx-table:
    cd tools/shadcn-ref && bun install --frozen-lockfile
    bun run tools/twmerge/gen.ts tools/shadcn-ref/node_modules/tailwind-merge/src cxtable.go
    gofmt -w cxtable.go

# Rebuild the test cases of gx.Cx from the test suite of the pinned
# tailwind-merge release (REQ-STY-04). Needs network: the tests are not in
# the npm package.
cx-cases:
    bash tools/twmerge/cases.sh

# Check the user docs against the rules of ASD-STE100 that a program can
# decide (NFR-10).
ste-lint:
    go run ./internal/stelint/cmd/stelint docs/content README.md

evidence:
    go run ./internal/build/evidence --write

evidence-check:
    go run ./internal/build/evidence --check

# Registry accessibility: zero serious or critical axe violations (REQ-REG-09).
a11y:
    bash tests/e2e/record.sh a11y.spec.ts

# The fallback path of partly supported platform features (REQ-REG-13).
fallback:
    bash tests/e2e/record.sh ./fallback.browsers.ts
