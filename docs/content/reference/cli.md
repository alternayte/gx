---
title: "The gx command"
description: "Each command of gx, with its arguments."
section: Reference
order: 2
---

Run the command of the project with `go run ./cmd/gx <command>`. It uses the Gx version in `go.mod`. A command that takes an app directory uses the current directory when you give none.

## gx init

```sh
gx init [--template app|docs] [--adapter datastar|htmx] [--module <path>] [dir]
```

Writes a new app into `dir`. It asks for the adapter when you give no `--adapter`. It does not write into a directory that has a `go.mod` or a `gx.toml`.

| Flag | Meaning |
| --- | --- |
| `--template` | `app` is an app with one example slice. `docs` is a docs site with the docs shell and the docs kit. |
| `--adapter` | `datastar` is the default. `htmx` gives an app with no signals. Read [Adapters](/guides/adapters/). |
| `--module` | The Go module path. The default is the directory name. |
| `--registry` | The component registry for `gx.toml`. |

## gx new

```sh
gx new page|action|form|component|slice <name> [app]
```

Adds typed code. A page, an action and a form take `<slice>/<Name>`. A component takes `<dir>/<Name>`. A slice takes a package name.

The command writes each file or none, adds the new value to the route list of its slice, and generates the code.

## gx dev

```sh
gx dev [--addr 127.0.0.1:3333] [--main <package>] [app]
```

Runs the app with build, restart, page morph and the error overlay.

## gx check

```sh
gx check [--json] [--external-links] [app]
```

Checks each `.gx` file, the routes, the actions, the forms and the content. It fails when a generated file is stale. It type-checks each island with the pinned TypeScript compiler. `--json` prints the diagnostics for a tool. `--external-links` also requests each link to a different site.

## gx generate

```sh
gx generate [app]
```

Writes the generated Go files. Commit them.

## gx fmt

```sh
gx fmt [--check] [file ...]
```

Formats `.gx` files in place. With no file, it formats standard input. `--check` fails on a file that is not formatted.

## gx lint

```sh
gx lint [--json] [app]
```

Runs `go vet` and the Gx analyzers on each package.

## gx build

```sh
gx build [-o <file>] [--main <package>] [app]
```

Generates the code, builds the stylesheet and builds the app. The default output is `bin/<name of the main package>`.

## gx export

```sh
gx export [--out dist] [--site <url>] [--main <package>] [app]
```

Renders each `GET` page to a static file. It stops when the app has a feature that needs a server.

## gx routes

```sh
gx routes [--json] [app]
```

Prints each route with its type, its page, its fields, its prefix, its layouts and its middleware.

## gx describe

```sh
gx describe [--json] [--schema] [app]
```

Prints the app model. `--schema` prints the JSON Schema of the JSON output.

## gx add

```sh
gx add [--registry <source>] [--dir <dir>] <item>[@version] [app]
```

Installs a registry item and the items that it needs. `@name/item` reads from a named registry.

## gx diff

```sh
gx diff [--registry <source>] <item> [app]
```

Shows what changed in your copy of an item and in the registry.

## gx update

```sh
gx update [--registry <source>] <item>[@version] [app]
```

Merges the registry version of an item with your copy.

## gx registry

```sh
gx registry build [--out <dir>] <source>
gx registry lint <source>
```

`build` writes a registry that you can publish. `lint` fails an item that lacks a required part.

## gx icons

```sh
gx icons pin <set>@<version> [app]
```

Pins an icon set and writes one component for each icon.

## gx pin

```sh
gx pin [--cdn <url>] <package>@<version> [app]
```

Stores the bundled ES module of an npm package in `js/vendor`, with each module that it imports. `gx.lock` records the hash of each file. An island then imports the package by its name. A changed file stops the build.

## gx wc

```sh
gx wc pin [--as <name>] [--out <dir>] [--element <tag>] [--cdn <url>] <package>@<version> [app]
```

Reads the custom elements manifest of an npm package and writes a Go package with one typed tag for each element. It pins the module of each element as `gx pin` does. `--element` limits the import to one tag; give it again for more tags.

## gx vendor

```sh
gx vendor [app]
```

Stores the pinned downloads in `.gx/vendor`, for a build with no network: Tailwind, Pagefind, the TypeScript compiler and the icon packs.

## gx import

```sh
gx import starlight --out <dir> <source>
```

Converts a Starlight project into Gx content.

## gx agents

```sh
gx agents --update [app]
```

Writes the current text into the managed section of `AGENTS.md`.

## gx lsp

```sh
gx lsp [--root <dir>]
```

Runs the language server on standard input and output.

## gx mcp

```sh
gx mcp [--main <package>] [--registry <source>] [app]
```

Runs the dev MCP server on standard input and output.

## gx help

```sh
gx help
```

Prints the list of commands.
