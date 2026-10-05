# tree-sitter-gx

The tree-sitter grammar for `.gx` files (REQ-TLS-06).

The grammar is a fork of [tree-sitter-html](https://github.com/tree-sitter/tree-sitter-html)
(MIT, Max Brunsfeld and Amaan Qureshi). It follows `docs/content/reference/grammar.md` and the
parser in `internal/compiler`. A tag name keeps its case, and only a void
element ends with no closing tag. The additions are:

- `package_clause` and `import_declaration`: the file header.
- `go_block`: the `props` and `signals` blocks, whose bodies are Go. A
  `line_comment` above a field is its description; a `go_string` holds any
  character.
- `expression`: a `{...}` expression. Its body is `go_code`; a brace in a
  string or in a comment does not close it.
- `if_statement`, `for_statement` and `switch_statement`: control lines. The
  header is `go_code`.
- `statement`: a `name := expr` line.
- `fragment_attribute`: `#name` and `#name(params)`.
- `spread_attribute`: `{...expr}`.

The external scanner in `src/scanner.c` reads the tokens that depend on the
line: control keywords, statement lines, the closing brace of a block and
text.

`queries/highlights.scm` marks component tags, fragments, signals, keywords
and directives. `queries/injections.scm` switches to Go for expressions,
control headers, statements and the props and signals blocks, and to
TypeScript, JavaScript and CSS for scripts and styles.

The parser in `src/` is generated with tree-sitter 0.25.10 (ABI 14).

```sh
npx tree-sitter-cli@0.25.10 generate
npx tree-sitter-cli@0.25.10 parse --quiet ../../registry/*/*.gx
```

`checks/tree-sitter.sh` fails when `src/parser.c` is stale or when a tracked
`.gx` file parses with an ERROR or MISSING node.
