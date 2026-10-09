---
title: "Content sites"
description: "Markdown collections with typed frontmatter, components in Markdown, the docs kit and search."
section: Guides
order: 9
---

A collection is a directory of Markdown files. Each file is one page. The frontmatter is a typed Go struct, and a page can use `.gx` components.

For a full docs site, start from the docs template. It has the docs shell, the docs kit, the sidebar and the search.

```sh
gx init --template docs mydocs
```

The rest of this page shows the parts on a small collection.

## A collection

`gx.Collection[Meta](dir)` reads the files of a directory. `Meta` is the type of the frontmatter. A key that is not a field of `Meta` is the diagnostic [GX8001](/errors/GX8001/).

`gx.ContentEntries(collection, view)` makes one route for each file. The view gets the entry: the slug, the frontmatter and the Markdown body.

```go title="notes/notes.go"
// Package notes is a small content collection.
package notes

import (
	"github.com/alternayte/gx"
	"github.com/alternayte/gx/content"
)

// Meta is the frontmatter of one note.
type Meta struct {
	Title string `yaml:"title"`
	// Order sorts the notes in the list.
	Order int `yaml:"order"`
}

// Notes is the collection.
var Notes = gx.Collection[Meta]("content/notes")

// view renders one note.
func view(e gx.Entry[Meta]) gx.Node {
	body, err := content.Body(e.Body)
	if err != nil {
		body = gx.Text(err.Error())
	}
	return Note(NoteProps{Title: e.Meta.Title, Children: body})
}

// Routes serves one page for each Markdown file.
var Routes = gx.Collect(gx.ContentEntries(Notes, view))
```

```gx title="notes/Note.gx"
package notes

props {
  // Title is the title of the note.
  Title string
  // Children is the rendered Markdown.
  Children gx.Node
}

<gx.Head title={p.Title} />
<article class="gx-content">
  <h1 class="text-2xl font-semibold">{p.Title}</h1>
  {p.Children}
</article>
```

```markdown title="content/notes/first.md"
---
title: First note
order: 1
---

## A heading

A note with a [link to the second note](/notes/second/) and a table.

| Tool | Use |
| --- | --- |
| `gx check` | Finds a broken link. |
```

```markdown title="content/notes/second.md"
---
title: Second note
order: 2
---

Back to the [first note](/notes/first/#a-heading).
```

Mount the routes under a prefix. `content.Install()` connects the YAML decoder and the Markdown renderer.

```go title="cmd/app/main.go"
// Command app serves the acme app.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/alternayte/gx"
	"github.com/alternayte/gx/adapters/datastar"
	"github.com/alternayte/gx/content"
	"acme/app"
	"acme/gxislands"
	"acme/gxstyles"
	"acme/home"
	"acme/notes"
)

func main() {
	setupGallery()
	content.Install()
	gx.SetStylesheet(gxstyles.CSS())
	gx.SetIslands(gxislands.Bundle())
	server := gx.New(gx.Config{Adapter: datastar.Adapter()})
	server.Group("/", app.Layout, gx.Nav(gx.MorphNavigation), home.Routes)
	server.Group("/notes", app.Layout, notes.Routes)

	addr := os.Getenv("GX_DEV_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	log.Printf("acme listens on http://%s", addr)
	log.Fatal(http.ListenAndServe(addr, server))
}
```

<Result page="guides/content-sites" get="/notes/first/" />

<!-- expect GET /notes/first/
<h2 id="a-heading">
href="/notes/second/"
<table>
-->

## Markdown

A page is CommonMark with the GitHub additions: tables, task lists, strikethrough, autolinks and footnotes. Each heading gets an id and an anchor link.

`gx check` checks each link to a page and each link to a heading. A site path holds the prefix of the `Group` call, as `/notes/second/` does here. A broken link is the diagnostic [GX8003](/errors/GX8003/). `gx check --external-links` also requests each link to a different site.

## Code blocks

A code block is highlighted when the site is built. The page ships no highlighter script.

| Option | Example | Effect |
| --- | --- | --- |
| A title | ` ```go title="main.go" ` | A frame with the file name. |
| Line marks | ` ```go {3-5} ` | Marks the lines. |
| A shell language | ` ```sh ` | A terminal frame. |

Each code block has a copy button.

## Components in Markdown

A page can use the components that the collection names, for example `gx.Collection[Meta]("content/docs").Components(docs.Aside, docs.Steps)`. A tag for a different component is the diagnostic [GX8002](/errors/GX8002/). The props are type-checked, as in a `.gx` file.

For a collection variable `Docs` with a component list, Gx generates the function `DocsBody(entry)`. It renders the Markdown with the components as typed calls. The docs template uses it.

## The docs kit and the docs shell

The registry has two items for a docs site.

| Item | Content |
| --- | --- |
| `docs` | Aside, Tabs and TabItem, Steps, Card, CardGrid, LinkCard, LinkButton, Badge, FileTree, Code. |
| `docs-shell` | The header, the sidebar, the table of contents, the page links, the theme select, the search dialog, the splash page and the 404 page. |
| `starlight` | A theme: one stylesheet with the colours, the fonts, the page layout, the prose and the code frames of the default Starlight theme. |
| `starlight-shell` | The page structure of the default Starlight theme. Use it with the `starlight` theme in place of `docs-shell`. |

Tabs with the same `sync` key change together, and the browser remembers the choice.

## Search

The search uses the Pagefind binary. Gx pins it in `gx.lock`, as it does with Tailwind. `gx export` builds the index from the exported pages, and `gx dev` builds it on the first search.

## Move a Starlight site

```sh
gx import starlight --out content/docs ../my-starlight-site
```

The command converts the frontmatter, the sidebar, the `.mdx` files and the Starlight components. It prints each part that it cannot convert.

To keep the look of the Starlight site, add the shell. The command also adds the theme and the docs kit:

```sh
gx add starlight-shell
```

Then import `./starlight.css` in `app/theme.css`, and render each page with `starlight.Shell`. The pages of the two items show each step.

Do these parts by hand after the import:

- The hero of a splash page. Write its actions in Go, as `docs.LinkButton` values in the `Actions` of `starlight.Hero`.
- The social links, the edit link and the versions. Set them in `starlight.Config`.
- A script of the `head` option, such as an analytics script. Render a `script` element after the shell.
