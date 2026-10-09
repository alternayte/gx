---
title: "The registry"
description: "Copy components into the app, own them, and merge later releases."
section: Guides
order: 6
---

The registry holds the components of Gx. They have the names, the look and the tokens of shadcn/ui, and they use native HTML elements in place of a JavaScript library.

`gx add` copies the source of an item into the app. The app owns the copy. `gx update` merges a later release with your changes.

## Add an item

```sh
gx add badge
```

The command does these steps.

1. It resolves the items that the item needs.
2. It checks the hash of each file against the registry index.
3. It writes the files to `ui/<name>/`.
4. It records each file in `gx.lock`.
5. It stores a snapshot in `.gx/base/<item>@<version>/`. Commit this directory.

`gx add` does not run code from the registry.

```gx title="home/Home.gx"
package home

import "acme/ui/badge"

<gx.Head title="Badges" />
<div class="flex gap-2">
  <badge.Badge>New</badge.Badge>
  <badge.Badge variant={badge.Outline}>Draft</badge.Badge>
</div>
<Counter label="Clicks" />
```

<Result page="guides/registry" get="/" />

<!-- expect GET /
data-slot="badge"
data-variant="outline"
-->

## What an item holds

| Part | Content |
| --- | --- |
| `.gx` files | The components. |
| `styles.go` | The variants as `gx.Enum` maps. |
| `<Name>.fixtures.go` | Named prop sets. The dev gallery and the docs show each one. |
| `USAGE.md` | Examples, the dos and the do nots, and the keyboard table. |

An item is a component, a block or a theme. A block is a larger part, such as a login page or an app shell.

## Three kinds of components

| Kind | How it works |
| --- | --- |
| Markup only | HTML and classes. It ships no JavaScript. |
| Native elements with signals | `<dialog>`, the Popover API, `<details>`, native `<select>`, and a small behaviour module for keys and focus. |
| Replaced by a Gx design | The form uses typed fields and rules. The data table sorts and pages on the server. The toast comes from the server with `c.Toast`. |

A page loads only the behaviour modules that its components use. A page with markup-only components loads no JavaScript.

## See what changed

```sh
gx diff badge
```

`gx diff` compares three versions: your copy, the snapshot, and the registry. It prints one status for each file: `unchanged`, `local`, `upstream`, `merged`, `conflict`, `upstream-added`, `upstream-removed` or `local-removed`.

## Merge a later release

```sh
gx update badge
```

`gx update` does a three-way merge of each file. A conflict gets conflict marks in the file, and the command reports it. The command does not replace a file with no notice.

## More than one registry

`gx.toml` names the registries.

```toml
[registry]
url = "https://raw.githubusercontent.com/alternayte/gx/v0.1.0/registry"
dir = "ui"

[registries.acme]
url = "https://ui.acme.example"
headers = "Authorization: Bearer TOKEN"
```

`gx add @acme/button` reads the item from the registry `acme`. The items that it needs come from the same registry.

## Publish a registry

A registry is static files: `index.json` and `items/<name>.json`. Any static host can serve it.

```sh
gx registry lint ./items
gx registry build --out ./public ./items
```

The source of an item is a directory with a `gx-item.json` file, the component files and a `USAGE.md` file. `gx registry lint` fails an item that lacks fixtures, usage sections, a keyboard table or a prop description. `gx registry build` writes the index, the items with their hashes, and the JSON Schema.
