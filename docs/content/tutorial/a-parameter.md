---
title: "2. A page with a parameter"
description: "Read a path value, link with a route value and answer 404."
section: Tutorial
order: 2
sample: tutorial
---

This step adds a page for one product.

## Make the page

```sh
go run ./cmd/gx new page shop/Detail
```

The command adds the route type `Detail`, writes `shop/detail_page.go` and `shop/DetailView.gx`, and adds `DetailPage` to the route list.

## A path variable

Change the pattern to `/shop/{id}`. A variable binds to the field with the same name. Upper and lower case are equal. A variable with no field is the diagnostic [GX3001](/errors/GX3001/).

```go title="shop/route/route.go"
// Package route holds the route types of the shop slice. A route
// package holds only route types.
package route

import "github.com/alternayte/gx"

// Index is the first page of the slice.
type Index struct {
	gx.Route `GET /shop`
}

// Detail is the page of one product. The variable {id} fills ID.
type Detail struct {
	gx.Route `GET /shop/{id}`
	ID       int64
}
```

## A loader that can fail

The loader returns `gx.NotFound()` when no product has the id. The app then answers 404.

```go title="shop/detail_page.go"
package shop

import (
	"github.com/alternayte/gx"
	"acme/shop/route"
)

// DetailPage is the page of one product.
var DetailPage = gx.Page(func(c *gx.Ctx, in route.Detail) (DetailViewProps, error) {
	product, ok := find(in.ID)
	if !ok {
		return DetailViewProps{}, gx.NotFound()
	}
	return DetailViewProps{Product: product}, nil
}, DetailView)
```

A value that is not a number, such as `/shop/tea`, answers 400. The loader does not run.

## A typed link

A link is a value of a route type. Gx writes the address from the pattern and the fields.

```gx title="shop/DetailView.gx"
package shop

import "acme/shop/route"

props {
  // Product is the product to show.
  Product Product
}

<gx.Head title={p.Product.Name} />
<a href={route.Index{}} class="text-sm text-muted-foreground">All products</a>
<h1 class="mt-2 text-2xl font-semibold">{p.Product.Name}</h1>
<p class="mt-2">{p.Product.Price} EUR</p>
```

The list links to each product.

```gx title="shop/IndexView.gx"
package shop

import "acme/shop/route"

props {
  // Products is the list to show.
  Products []Product
}

<gx.Head title="Shop" />
<h1 class="text-2xl font-semibold">Shop</h1>
<ul class="mt-4 grid gap-2">
  for _, it := range p.Products {
    <li class="rounded-md border border-border p-3">
      <a href={route.Detail{ID: it.ID}}>{it.Name}</a>: {it.Price} EUR
    </li>
  }
</ul>
```

```text title="GET /shop"
href="/shop/2"
```

```text title="GET /shop/2"
Coffee
href="/shop"
```

When you rename the field `ID` or change the pattern, each link changes with it or stops the build. A string in `href` that is not a constant is the diagnostic [GX2011](/errors/GX2011/).

Next: [an action and a signal](/tutorial/an-action/).
