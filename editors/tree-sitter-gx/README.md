# tree-sitter-gx

The tree-sitter grammar for `.gx` files (REQ-TLS-06).

The grammar is a fork of [tree-sitter-html](https://github.com/tree-sitter/tree-sitter-html)
(MIT, Max Brunsfeld and Amaan Qureshi) with three additions:

- `expression`: a `{...}` Go expression, nested braces included.
- `go_block`: the `props` and `signals` blocks, whose bodies are Go. A
  `line_comment` above a field is its description; a `go_string` holds any
  character.
- `fragment_attribute`: `#name` and `#name(params)`.

`queries/highlights.scm` marks component tags, fragments, signals and
directives. `queries/injections.scm` switches to Go for expressions and the
props and signals blocks, and to TypeScript, JavaScript and CSS for scripts
and styles.

The parser in `src/` is generated with tree-sitter 0.25.10 (ABI 14).

```sh
npx tree-sitter-cli@0.25.10 generate
npx tree-sitter-cli@0.25.10 test
```
