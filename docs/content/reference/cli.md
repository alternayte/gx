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

`gx add theme:<name>` writes a theme of a plugin of the project as `app/theme.css`. When the app changed its theme, the command writes `app/theme.<name>.css` and leaves the theme of the app.

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

```sh
gx wc build [--server <origin>] [--base <path>] [--out <dir>] [app or package]
```

Writes the files of each widget for a host page into `dist/widgets`: the element file, a `.d.ts` file, a file with the JSX types for React, and one `custom-elements.json`. With a package directory it writes the widgets of that package only. `--server` is the origin of the Gx server and `--base` the base path of the app; each has the key of the same name in `[widgets]` of `gx.toml` as its default. With no server the element calls the origin of its host page.

```sh
gx wc check [--update] [app]
```

Compares the contract of the widgets with the baseline in `.gx/base/widgets.json`, and prints each change. A removed or retyped attribute, event, event detail field or CSS variable needs a major bump of the `version` in `[widgets]`. An addition needs a minor bump. The command fails when the version does not have the bump. With no baseline each version passes. `--update` records the contract of now as the baseline after the check passes.

```sh
gx wc pack [--server <origin>] [--base <path>] [--out <dir>] [app]
```

Writes the npm tarball of the widgets into `dist/widgets`: the files of `gx wc build` and a `package.json`. `[widgets]` of `gx.toml` gives the `name` and the `version` of the package.

```sh
gx wc publish [--server <origin>] [--base <path>] [--registry <url>] [--tag <name>] [app]
```

Runs the check of `gx wc check`, then publishes the package through the HTTP API of the npm registry. The environment variable `NPM_TOKEN` holds the token of the registry. The keys `registry` and `access` of `[widgets]` set the registry and `public` or `restricted`. After the registry takes the version, the command writes the baseline. No node runs.

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
