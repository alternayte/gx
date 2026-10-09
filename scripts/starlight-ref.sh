#!/usr/bin/env bash
# Refreshes tests/e2e/starlight-ref: the pages of the built Deedbox docs
# (Starlight, MIT) that `just parity-starlight` compares the example with.
# Build the Deedbox site first (npm run build in its site directory), then:
#
#   scripts/starlight-ref.sh ~/Development/deedbox/site
set -euo pipefail
site="${1:?usage: scripts/starlight-ref.sh <deedbox site directory>}"
dist="$site/dist"
if [ ! -f "$dist/index.html" ]; then
  echo "starlight-ref: $dist has no index.html; build the site first" >&2
  exit 1
fi
root="$(git rev-parse --show-toplevel)"
out="$root/tests/e2e/starlight-ref"
rm -rf "$out"
mkdir -p "$out"
cp -R "$dist/_astro" "$out/_astro"
cp "$dist/index.html" "$dist/404.html" "$out/"
# The same pages as the list in tests/e2e/starlight.browsers.ts.
for page in tutorials/first-stream how-to/add-a-projection reference/configuration reference/errors/dbx001; do
  mkdir -p "$out/$page"
  cp "$dist/$page/index.html" "$out/$page/index.html"
done
cat > "$out/README.md" <<'NOTE'
# Starlight reference pages

These files are pages of the built Deedbox docs site, which uses the default
Starlight theme. `just parity-starlight` compares `examples/deedbox-docs`
with them. `scripts/starlight-ref.sh` writes them; do not edit them.

The Deedbox docs are MIT licensed: https://github.com/alternayte/deedbox
Starlight is MIT licensed: https://github.com/withastro/starlight
NOTE
echo "starlight-ref: wrote $(find "$out" -type f | wc -l | tr -d ' ') files to $out"
