#!/usr/bin/env bash
# check: tree-sitter
# born: 2026-10-05
# failure: the tree-sitter grammar reported syntax errors on .gx files the compiler accepts
# rule: the committed parser is generated from grammar.js and parses every tracked .gx file with no ERROR or MISSING node
set -euo pipefail
root="$(git rev-parse --show-toplevel)"
cd "$root"
grammar="editors/tree-sitter-gx"
if ! command -v npx >/dev/null 2>&1; then
  echo "tree-sitter: npx is not on PATH; the grammar cannot be checked" >&2
  exit 1
fi
if ! command -v cc >/dev/null 2>&1; then
  echo "tree-sitter: cc is not on PATH; the parser cannot be compiled" >&2
  exit 1
fi
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
# The pinned CLI comes from the npm cache. npm uses the network only when
# the cache does not hold it.
ts() { npx --yes --prefer-offline tree-sitter-cli@0.25.10 "$@"; }

(cd "$grammar" && ts generate --abi 14 --output "$tmp/gen" grammar.js) >"$tmp/generate.log" 2>&1 || {
  cat "$tmp/generate.log" >&2
  echo "rule: $grammar/grammar.js does not generate" >&2
  exit 1
}
for f in parser.c grammar.json node-types.json; do
  if ! cmp -s "$tmp/gen/$f" "$grammar/src/$f"; then
    echo "rule: $grammar/src/$f is stale; run npx tree-sitter-cli@0.25.10 generate in $grammar" >&2
    exit 1
  fi
done

# The compiled parser goes to a scratch directory: the default cache is
# shared by every checkout of the grammar.
git ls-files -z '*.gx' | while IFS= read -r -d '' f; do
  printf '%s\n' "$root/$f"
done >"$tmp/paths"
# npx gives exit code 0 when the parser stops with a signal, and a quiet
# parse then prints nothing. One parse with output proves that the parser
# runs: its tree starts with the document node.
first="$(head -n 1 "$tmp/paths")"
(cd "$grammar" && TREE_SITTER_LIBDIR="$tmp/lib" ts parse "$first") >"$tmp/first.out" 2>"$tmp/first.err" || true
if ! grep -q '^(document' "$tmp/first.out"; then
  # The output of the parse says why: a compile error of the parser, or
  # nothing for a parser that a signal stopped.
  head -c 2000 "$tmp/first.out" "$tmp/first.err" >&2 || true
  echo "rule: the tree-sitter parser gives no tree for $first; it did not run or it crashed" >&2
  exit 1
fi
if ! (cd "$grammar" && TREE_SITTER_LIBDIR="$tmp/lib" ts parse --quiet --paths "$tmp/paths") >"$tmp/parse.log" 2>&1; then
  grep -E 'ERROR|MISSING' "$tmp/parse.log" | sed "s|^$root/||" >&2 || cat "$tmp/parse.log" >&2
  echo "rule: the tree-sitter grammar must parse every .gx file the compiler accepts; fix $grammar/grammar.js or $grammar/src/scanner.c" >&2
  exit 1
fi
