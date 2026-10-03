#!/usr/bin/env bash
# check: vscode-smoke
# born: 2026-10-04
# failure: the VS Code extension fails to open, diagnose, complete or format a .gx file
# rule: REQ-TLS-05: run @vscode/test-electron against a scratch app with the built gx binary
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

if ! command -v node >/dev/null 2>&1; then
  echo "vscode-smoke: node is required for @vscode/test-electron; install node and retry" >&2
  exit 1
fi
if ! command -v bun >/dev/null 2>&1; then
  echo "vscode-smoke: bun is required to install the extension test deps" >&2
  exit 1
fi

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

bin="$work/gx"
go build -o "$bin" ./cmd/gx

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

cd editors/vscode
if [ ! -d node_modules ]; then
  bun install --no-save
fi
GX_SERVER_PATH="$bin" GX_VSCODE_FIXTURE="$fixture" node ./test/run.js
