---
title: "Routing"
description: "Route types, query values, typed links, layouts, errors and navigation."
section: Guides
order: 2
---

A route is a Go struct type. It embeds `gx.Route`, and its tag holds the method and the pattern. The pattern uses the syntax of the Go `ServeMux`.

A route type lives in the `route` package of its slice. That package holds only route types, so each slice can link to each other slice with no import cycle.

```sh
go run ./cmd/gx new slice shop
```

## Path and query values

A path variable binds to the field with the same name. A query field has a `query` tag and can have a `default` tag.

```go title="shop/route/route.go"
// Package route holds the route types of the shop slice. A route
// package holds only route types.
package route

import "github.com/alternayte/gx"

// Index is the list of products. Sort and Page come from the query.
type Index struct {
	gx.Route `GET /shop`
	Sort     string `query:"sort" default:"name"`
	Page     int    `query:"page" default:"1"`
}
```

A query value of the wrong type, such as `?page=two`, answers 400. The loader does not run.

A default applies when the request has no value for the field. A value in the request stays, also when it is the zero value: `?page=0` binds 0.

`GET /` matches each path that no other pattern matches. Write `GET /{$}` for the root path only, as the home page of a new app does.

## Pages

`gx.Page(load, view)` joins a loader and a view. The loader is `func(*gx.Ctx, In) (Props, error)`. The view is the component function.

```go title="shop/shop.go"
// Package shop is the shop slice: its pages, actions and views.
package shop

import (
	"github.com/alternayte/gx"
	"acme/shop/route"
)

// IndexPage is the list of products.
var IndexPage = gx.Page(func(c *gx.Ctx, in route.Index) (IndexViewProps, error) {
	if in.Page < 1 {
		return IndexViewProps{}, gx.NotFound()
	}
	return IndexViewProps{Sort: in.Sort, Page: in.Page}, nil
}, IndexView)

// Routes lists every page, action and form of the slice.
var Routes = gx.Collect(IndexPage)
```

A loader returns `gx.NotFound()`, `gx.Forbidden()` or `gx.Redirect(route)` to stop with a status.

## Typed links

`href` takes a static string, a value of a `GET` route type, or a `gx.URL` value. Gx escapes the path values and encodes the query. It omits a query value that is equal to its default.

`active="section"` marks a link when the current path starts with the link path. A link to the exact current page gets `aria-current="page"`.

```gx title="shop/IndexView.gx"
package shop

import "acme/shop/route"

props {
  // Sort is the sort key.
  Sort string
  // Page is the page number.
  Page int
}

<gx.Head title="Shop" />
<nav class="flex gap-3 text-sm">
  <a href={route.Index{}}>By name</a>
  <a href={route.Index{Sort: "price"}}>By price</a>
  <a href={route.Index{Sort: "price", Page: 2}}>Page 2</a>
</nav>
<p class="mt-4">Sorted by {p.Sort}, page {p.Page}.</p>
```

```text title="GET /shop?sort=price"
Sorted by price, page 1.
href="/shop?sort=price"
href="/shop"
```

## Mount

Mounting is explicit. `gx.Collect` lists the pages, actions and forms of a slice. `Group` mounts lists under a prefix, with layouts and middleware. A value that no `gx.Collect` holds is the diagnostic [GX3003](/errors/GX3003/).

Middleware is `func(http.Handler) http.Handler`. It applies to the lists that follow it in the same call.

```go title="cmd/app/main.go"
// Command app serves the acme app.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/alternayte/gx"
	"github.com/alternayte/gx/adapters/datastar"
	"acme/app"
	"acme/gxstyles"
	"acme/home"
	"acme/shop"
)

// noStore is plain net/http middleware.
func noStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func main() {
	setupGallery()
	gx.SetStylesheet(gxstyles.CSS())
	server := gx.New(gx.Config{Adapter: datastar.Adapter()})
	server.Group("/", app.Layout, gx.Nav(gx.MorphNavigation), noStore, home.Routes, shop.Routes)

	addr := os.Getenv("GX_DEV_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	log.Printf("acme listens on http://%s", addr)
	log.Fatal(http.ListenAndServe(addr, server))
}
```

## Layouts

A layout is a component with a `Children` prop, plus an optional loader. The layout loader and the page loader run at the same time. `gx.Once(c, fn)` runs `fn` one time for each request, so a layout and a page can share one query.

```go title="app/layout.go"
// Package app holds the layout and the theme of the app.
package app

import "github.com/alternayte/gx"

// Layout wraps every page in the shell. The loader gets the name of the
// reader.
var Layout = gx.Layout(func(c *gx.Ctx) (string, error) {
	return gx.Once(c, func() (string, error) { return "Ada", nil })
}, func(user string, children gx.Node) gx.Node {
	return Shell(ShellProps{User: user, Children: children})
})
```

```gx title="app/Shell.gx"
package app

import (
  homeroute "acme/home/route"
  shoproute "acme/shop/route"
)

props {
  // User is the name of the reader.
  User string
  // Children is the page.
  Children gx.Node
}

<gx.Head title="Acme" bodyClass="bg-background text-foreground" />
<header class="border-b border-border">
  <nav class="mx-auto flex max-w-3xl items-center gap-4 p-4 text-sm">
    <a href={homeroute.Home{}} class="font-semibold">Acme</a>
    <a href={shoproute.Index{}} active="section">Shop</a>
    <span class="ml-auto text-muted-foreground">{p.User}</span>
  </nav>
</header>
<main class="mx-auto max-w-3xl p-4">{p.Children}</main>
```

`<gx.Head>` sets the title, the meta tags and the link tags. The title of the page replaces the title of the layout.

```text title="GET /shop"
<title>Shop</title>
Ada
data-active
```

## Navigation

With `gx.Nav(gx.MorphNavigation)`, a click between two pages with a shared layout fetches only the part inside that layout. The layout elements stay in the page. The title and the address change, and the back button works.

With no JavaScript, or between two different layouts, a link is a full load.

## Error pages

`server.Errors(notFound, forbidden, serverError)` sets a component for each status. Each one is `func(*gx.Ctx) gx.Node`.

## Use Gx with an existing router

Gx is plain `net/http`. Adopt it one level at a time.

| Level | What you write |
| --- | --- |
| 1. One component | `gx.Render(w, r, node)` in any handler. |
| 2. One route | `mux.Handle(route.Index{}.Pattern(), shop.IndexPage)`. Each page, action and form is an `http.Handler`. |
| 3. One mounted app | Mount a `gx.App` under a prefix and set `Config.BasePath`. Each link then starts with the prefix. |
| 4. The whole app | `gx.New` and `Group`, as above. |

```go title="legacy/legacy.go"
// Package legacy shows Gx inside handlers that do not use gx.App.
package legacy

import (
	"net/http"

	"github.com/alternayte/gx"
	"acme/shop"
	"acme/shop/route"
)

// Banner renders one component in a plain handler (level 1).
func Banner(w http.ResponseWriter, r *http.Request) {
	_ = gx.Render(w, r, shop.IndexView(shop.IndexViewProps{Sort: "name", Page: 1}))
}

// Mount adds one Gx page to a standard mux (level 2).
func Mount(mux *http.ServeMux) {
	mux.Handle(route.Index{}.Pattern(), shop.IndexPage)
}
```

A router that does not fill `r.PathValue` needs `gx.Params`, which tells Gx how to read a path value.

## List the routes

```sh
go run ./cmd/gx routes
go run ./cmd/gx routes --json
```
