#!/usr/bin/env bash
# Writes testdata/tailwind-merge-<version>.json: the default-config cases of
# the tailwind-merge test suite and of its documentation examples. The npm
# package holds no tests, so this downloads the release from GitHub and
# first proves that its src equals the pinned npm package.
set -euo pipefail
root="$(git rev-parse --show-toplevel)"
cd "$root"
(cd tools/shadcn-ref && bun install --frozen-lockfile >/dev/null)
pkg="tools/shadcn-ref/node_modules/tailwind-merge"
version="$(bun -e "console.log(require('./$pkg/package.json').version)")"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
curl -fsSL "https://codeload.github.com/dcastil/tailwind-merge/tar.gz/refs/tags/tailwind-merge%40$version" | tar xz -C "$tmp"
release="$(echo "$tmp"/*/packages/tailwind-merge)"
if ! diff -rq "$release/src" "$pkg/src" >/dev/null; then
  echo "the src of the GitHub release differs from the npm package $version" >&2
  exit 1
fi
bun run tools/twmerge/extract.ts "$release/tests" "$pkg/src" "$tmp/cases.json"
bun -e "
const d = JSON.parse(require('fs').readFileSync('$tmp/cases.json', 'utf8'))
const out = { source: 'tailwind-merge $version, packages/tailwind-merge/tests and the documentation examples (MIT, Copyright (c) 2021 Dany Castillo)', cases: d.cases }
require('fs').writeFileSync('testdata/tailwind-merge-$version.json', JSON.stringify(out, null, 1) + '\n')
"
echo "wrote testdata/tailwind-merge-$version.json"
