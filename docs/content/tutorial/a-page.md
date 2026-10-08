---
title: "1. A page"
description: "Add a slice with a route, a loader and a view."
section: Tutorial
order: 1
sample: tutorial
---

The tutorial builds a small shop in the app of the [quick start](/start/quick-start/). Each step adds one feature.

A slice is one feature package. It owns its routes, its handlers and its views.

## Make the slice

```sh
gx new slice shop
```

The command writes three files and mounts the slice in `cmd/app/main.go`.

| File | Content |
| --- | --- |
| `shop/route/route.go` | The route types of the slice. |
| `shop/shop.go` | The page and the route list. |
| `shop/IndexView.gx` | The view of the page. |

## The route type

A route is a Go struct. The tag holds the method and the pattern.

```go title="shop/route/route.go"
// Package route holds the route types of the shop slice. A route
// package holds only route types.
package route

import "github.com/alternayte/gx"

// Index is the first page of the slice.
type Index struct {
	gx.Route `GET /shop`
}
```

## The data

A real app reads a database. The tutorial uses a list in memory.

```go title="shop/products.go"
package shop

// Product is one product of the shop.
type Product struct {
	ID    int64
	Name  string
	Price int
}

// products is the data of the tutorial.
var products = []Product{
	{ID: 1, Name: "Tea", Price: 4},
	{ID: 2, Name: "Coffee", Price: 5},
	{ID: 3, Name: "Cocoa", Price: 6},
}

// find returns the product with the id.
func find(id int64) (Product, bool) {
	for _, product := range products {
		if product.ID == id {
			return product, true
		}
	}
	return Product{}, false
}
```

## The loader and the page

`gx.Page` joins a loader and a view. The loader gets the route input and returns the props of the view. A type that does not match is a Go compile error.

```go title="shop/shop.go"
// Package shop is the shop slice: its pages, actions and views.
package shop

import (
	"github.com/alternayte/gx"
	"acme/shop/route"
)

// IndexPage is the list of products. The loader returns the props of
// the view.
var IndexPage = gx.Page(func(c *gx.Ctx, in route.Index) (IndexViewProps, error) {
	return IndexViewProps{Products: products}, nil
}, IndexView)

// Routes lists every page, action and form of the slice.
var Routes = gx.Collect(IndexPage)
```

## The view

A `.gx` file is one component. The `props` block declares its data. The markup is HTML, and the logic is Go.

```gx title="shop/IndexView.gx"
package shop

props {
  // Products is the list to show.
  Products []Product
}

<gx.Head title="Shop" />
<h1 class="text-2xl font-semibold">Shop</h1>
<ul class="mt-4 grid gap-2">
  for _, it := range p.Products {
    <li class="rounded-md border border-border p-3">{it.Name}: {it.Price} EUR</li>
  }
</ul>
```

Open `http://127.0.0.1:3333/shop`.

```text title="GET /shop"
Tea: 4 EUR
Cocoa: 6 EUR
```

## What you have

- A route type that the compiler checks.
- A loader that does the IO and returns typed props.
- A view that renders from its props only.

Next: [a page with a parameter](/tutorial/a-parameter/).
