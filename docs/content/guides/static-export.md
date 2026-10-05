---
title: "Static export"
description: "Render the pages of an app to files for any static host."
section: Guides
order: 10
---

`gx export` renders an app to static files. Use it for a site that needs no server: a docs site, a landing page or a blog.

```sh
go run ./cmd/gx export --out dist
go run ./cmd/gx export --out dist --site https://docs.example.com
```

## What the export writes

| Output | Content |
| --- | --- |
| One `index.html` for each page | Each `GET` page with no path variable, and each input of a page with `.Static(...)`. |
| `_gx/` | The scripts and the stylesheet. Each file name holds a hash of its content. |
| The public files | Each file of `Config.Public`, with its own name. |
| `404.html` | The not-found page of the app. |
| `sitemap.xml`, `robots.txt` | Written when the site has a URL. |
| `llms.txt`, `llms-full.txt`, `llms-small.txt` and one `.md` for each page | Written for a content collection with `.LLMS(...)`. |
| `pagefind/` | The search index. |

## Pages with a path variable

The export cannot know the values of a path variable. `.Static(fn)` gives it the list of inputs.

```sh
go run ./cmd/gx new slice catalog
```

```go title="catalog/route/route.go"
// Package route holds the route types of the catalog slice. A route
// package holds only route types.
package route

import "github.com/alternayte/gx"

// Index is the first page of the slice.
type Index struct {
	gx.Route `GET /catalog`
}

// Show is the page of one item.
type Show struct {
	gx.Route `GET /catalog/{id}`
	ID       int64
}
```

```go title="catalog/catalog.go"
// Package catalog is the catalog slice: its pages and views.
package catalog

import (
	"github.com/alternayte/gx"
	"acme/catalog/route"
)

// IndexPage is the first page of the slice.
var IndexPage = gx.Page(func(c *gx.Ctx, in route.Index) (IndexViewProps, error) {
	return IndexViewProps{}, nil
}, IndexView)

// ShowPage is the page of one item. Static gives the export each id.
var ShowPage = gx.Page(func(c *gx.Ctx, in route.Show) (ShowViewProps, error) {
	if in.ID < 1 || in.ID > 3 {
		return ShowViewProps{}, gx.NotFound()
	}
	return ShowViewProps{ID: in.ID}, nil
}, ShowView).Static(func() ([]route.Show, error) {
	return []route.Show{{ID: 1}, {ID: 2}, {ID: 3}}, nil
})

// Routes lists every page of the slice.
var Routes = gx.Collect(IndexPage, ShowPage)
```

```gx title="catalog/ShowView.gx"
package catalog

import "acme/catalog/route"

props {
  // ID is the item.
  ID int64
}

<gx.Head title="Item" />
<h1 class="text-2xl font-semibold">Item {p.ID}</h1>
<a href={route.Index{}}>All items</a>
```

```text title="GET /catalog/2"
Item 2
```

## Features that need a server

An exported site has no server. The export stops, lists these features and writes nothing:

- an action, unless a different server answers it;
- a form;
- a field with live validation.

Navigation between pages with a shared layout becomes a full load in an exported site.

## An action on a different server

`.External(url)` marks an action that a different server answers. Each invocation of the action then calls that server, and the export can hold the page.

```go title="catalog/vote.go"
package catalog

import (
	"github.com/alternayte/gx"
	"acme/catalog/route"
)

// Vote runs on the API server. The static site only invokes it.
var Vote = gx.Action(func(c *gx.Ctx, in route.Vote) error {
	return c.Toast("Thank you")
}).External("https://api.example.com")
```

```go title="catalog/route/vote.go"
package route

import "github.com/alternayte/gx"

// Vote records one vote.
type Vote struct {
	gx.Route `POST /catalog/vote`
}
```

Add `Vote` to `gx.Collect` in the app that runs on the API server.

## Site data

The `[site]` table of `gx.toml` gives the data for the head of each page.

```toml
[site]
url = "https://docs.example.com"
title = "Acme"
title_template = "%s | Acme"
description = "The Acme docs."
```

The export writes the title, the description, the canonical link, the Open Graph tags and the Twitter card tags into each page.

## Images

An image in Markdown gets a hash in its file name, its `width` and `height`, and `loading="lazy"`. A PNG or JPEG image also gets smaller sizes in a `srcset`. The page does not move when the image loads.
