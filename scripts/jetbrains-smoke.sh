#!/usr/bin/env bash
# check: jetbrains-smoke
# born: 2026-10-07
# failure: the JetBrains plugin fails to start, highlight, diagnose or format a .gx file
# rule: REQ-TLS-07: run the IntelliJ Platform UI smoke test against a scratch app with the built gx binary
#
# GX_IDE_PATH names an installed IDE (GoLand or IntelliJ IDEA Ultimate) and
# GX_IDE_PRODUCT its product code (GO or IU). With no path, Gradle and the
# test framework download GoLand.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

if ! command -v java >/dev/null 2>&1; then
  echo "jetbrains-smoke: a JDK is required for Gradle; install JDK 21 and retry" >&2
  exit 1
fi

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

bin="$work/gx"
go build -o "$bin" ./cmd/gx

fixture="$work/fixture"
mkdir -p "$fixture/card"
cat > "$fixture/go.mod" <<MOD
module app

go 1.25.0

require github.com/alternayte/gx v0.0.0

replace github.com/alternayte/gx => $root
MOD
cp go.sum "$fixture/go.sum"
cat > "$fixture/card/Card.gx" <<'GX'
package card

props {
  Title string
}


<article>{p.Titel}</article>
GX

args=()
if [ -n "${GX_IDE_PATH:-}" ]; then
  args+=("-Pgx.ide.path=$GX_IDE_PATH" "-Pgx.ide.product=${GX_IDE_PRODUCT:-GO}")
fi
cd editors/jetbrains
GX_SERVER_PATH="$bin" GX_SMOKE_PROJECT="$fixture" GX_SMOKE_WORK="$work/ide" \
  ./gradlew integrationTest --console=plain "${args[@]}"
