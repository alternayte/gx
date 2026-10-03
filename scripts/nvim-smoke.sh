#!/usr/bin/env bash
# check: nvim-smoke
# born: 2026-10-04
# failure: headless Neovim cannot attach the Gx LSP, show diagnostics or format
# rule: REQ-TLS-06: open a .gx file in headless Neovim with the gx lsp attached
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

if ! command -v nvim >/dev/null 2>&1; then
  echo "nvim-smoke: nvim is required for this check" >&2
  exit 1
fi

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

go build -o "$work/gx" ./cmd/gx

fixture="$work/fixture"
mkdir -p "$fixture/card"
cat > "$fixture/go.mod" <<EOF
module app

go 1.25.0

require github.com/alternayte/gx v0.0.0

replace github.com/alternayte/gx => $root
EOF
cat > "$fixture/card/Card.gx" <<'EOF'
package card

props {
  Title string
}


<article>{p.Titel}</article>
EOF

cd "$fixture"
GX_BIN="$work/gx" \
  GX_NVIM_RTP="$root/editors/nvim" \
  GX_NVIM_FIXTURE="$fixture/card/Card.gx" \
  nvim -l "$root/editors/nvim/test/smoke.lua"
