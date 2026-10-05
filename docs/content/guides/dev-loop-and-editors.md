---
title: "The dev loop and editors"
description: "gx dev, the error overlay, the language server, formatting, lint and the debugger."
section: Guides
order: 8
---

## gx dev

```sh
go run ./cmd/gx dev
go run ./cmd/gx dev --addr 127.0.0.1:4000
```

`gx dev` is one command. It watches the files, generates the code, builds the stylesheet, builds the app with the `gxdev` tag, and runs the app behind a proxy. The proxy adds a small dev client to each page.

After a change, the command builds the app again and starts it. The browser then morphs the page: the scroll position, the input values and the signals stay.

| Change | What happens |
| --- | --- |
| A `.gx` file, a `.go` file or a Markdown content file | Generate, build with the Go build cache, start, morph. |
| `app/theme.css` or a class | Tailwind builds the stylesheet again. |

## The error overlay

A compile error, a type error, a render error or a panic shows in the browser as an overlay. The overlay has the message, the file, the line and a link that opens the file in your editor. The page comes back when you correct the error.

## Routes that exist in dev only

Each dev route is below `/_gx/`: the gallery, the export list and the dev channel. They exist only with the `gxdev` build tag. A binary from `gx build` does not have them.

## The language server

```sh
go run ./cmd/gx lsp
```

`gx lsp` speaks the Language Server Protocol on standard input and output, with no editor extension of its own.

- Diagnostics while you type, with the same codes as `gx check`.
- Completion of tags, props, attributes, Go expressions, routes in `href`, actions in `on:`, and signals.
- Hover with types and the generated signatures.
- Go to definition between `.gx` and `.go` files.
- Rename of props, fragments and signals.
- Formatting, semantic tokens, inlay hints and code actions.
- The same checks in Markdown content files.

## Editors

| Editor | Setup |
| --- | --- |
| VS Code, Cursor, VSCodium | The Gx extension in `editors/vscode`. It starts `gx lsp`, formats on save and has a `gx dev` task. |
| Neovim | The files in `editors/nvim`: the tree-sitter grammar, the filetype and the language server entry. Then `vim.lsp.enable("gx")`. |

## Format

```sh
go run ./cmd/gx fmt home/Home.gx
go run ./cmd/gx fmt --check home/Home.gx
```

`gx fmt` gives one form for each file. It formats the Go parts as `gofmt` does and sorts the imports. `--check` fails on a file that is not formatted.

## Go tools

The generated Go has `//line` comments that point at the `.gx` file. Each Go tool then reports a `.gx` position: the compiler, `go vet`, a panic, a stack trace and a coverage report.

```sh
go run ./cmd/gx lint
```

`gx lint` runs `go vet` and the Gx analyzers on each package. The analyzers are also a golangci-lint module plugin, in the package `github.com/alternayte/gx/lintplugin`.

| Code | Analyzer | Finding |
| --- | --- | --- |
| [GX3005](/errors/GX3005/) | `gxroutepkg` | A route package holds code other than route types. |
| [GX5001](/errors/GX5001/) | `gxenum` | A `gx.Enum` map misses a constant. |
| [GX5003](/errors/GX5003/) | `gxclassruntime` | A class string is made when the program runs. |
| [GX7001](/errors/GX7001/) | `gxsafehtml` | A value that is not a constant becomes `gx.SafeHTML`. |

## golangci-lint

Build a custom golangci-lint binary with the Gx plugin. The file `.custom-gcl.yml` names the module `github.com/alternayte/gx` and the import `github.com/alternayte/gx/lintplugin`.

```sh
golangci-lint custom
```

Then add the plugin to `.golangci.yml`.

```yaml
version: "2"

linters:
  default: standard
  enable:
    - gx
  settings:
    custom:
      gx:
        type: "module"
        description: Gx analyzers
        settings: {}
  exclusions:
    # The generated _gx.go files hold the findings for the .gx files.
    generated: disable
```

`generated: disable` keeps the generated files in the analysis. With the default value, golangci-lint skips them and the findings for `.gx` files do not show.

## Debug

Delve stops on a breakpoint that you set on a `.gx` line. It shows `p` and the locals. Build with `-gcflags "all=-N -l"` for the best result.
