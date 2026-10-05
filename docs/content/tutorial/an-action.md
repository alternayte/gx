---
title: "3. An action and a signal"
description: "Keep a value in the browser, send it to the server and patch one part of the page."
section: Tutorial
order: 3
sample: tutorial
---

This step adds a buy box to the product page. The amount is a signal in the browser. The **Add to cart** button invokes an action on the server.

## Make the action

```sh
go run ./cmd/gx new action shop/Add
```

## The input of the action

An action is a route type and a handler. The field `Qty` reads the signal `qty` of the component that invokes the action.

The browser controls each signal value. The `Rules` method checks the value before the handler runs. A signal field with no rule is the diagnostic [GX4008](/errors/GX4008/).

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

// Add puts a product in the cart. Qty comes from the Qty signal of the
// buy box.
type Add struct {
	gx.Route `POST /shop/{id}/cart`
	ID       int64
	Qty      int `signal:"qty"`
}

// Rules checks the amount.
func (in *Add) Rules() gx.Rules {
	return gx.Rules{gx.Field(&in.Qty, gx.Min(1), gx.Max(10))}
}
```

## The component

The `signals` block declares the state that lives in the browser. `$Qty` works in a client expression only: `bind:`, `show`, `text`, `class:`, `attr:` and `on:`.

`#status` marks a fragment. Gx generates the function `BuyStatus`, which renders this element alone.

```gx title="shop/Buy.gx"
package shop

import "acme/shop/route"

props {
  // ProductID is the product to buy.
  ProductID int64
}

signals {
  // Qty is the amount in the number field.
  Qty int = 1
}

<div class="mt-4 flex items-center gap-2">
  status := "Not in the cart"
  <input type="number" min="1" max="10" bind:value={$Qty} class="w-20 rounded-md border border-border px-2 py-1" />
  <button class="rounded-md bg-primary px-3 py-1.5 text-sm text-primary-foreground" on:click={route.Add{ID: p.ProductID}}>Add to cart</button>
  <span #status(status string) class="text-sm text-muted-foreground">{status}</span>
</div>
```

## The handler

The handler answers with a patch. `c.Patch` sends the fragment, and the browser changes that element only. The signal `Qty` and the number field keep their value.

A component with signals has a scope. `gx.ScopeKey` gives the key of the instance that invoked the action.

```go title="shop/add_action.go"
package shop

import (
	"fmt"

	"github.com/alternayte/gx"
	"acme/shop/route"
)

// Add puts a product in the cart and patches the status of the buy box
// that invoked it.
var Add = gx.Action(func(c *gx.Ctx, in route.Add) error {
	product, ok := find(in.ID)
	if !ok {
		return gx.NotFound()
	}
	key := gx.ScopeKey(gx.Scope(c.R), "shop.Buy")
	return c.Patch(BuyStatus(key, fmt.Sprintf("%d x %s is in the cart", in.Qty, product.Name)))
})
```

## Use the component

A tag with an upper-case name is a component of the same package. An attribute is a prop with a lower-case first letter.

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
<Buy productID={p.Product.ID} />
```

```text title="GET /shop/2"
Add to cart
Not in the cart
```

Open a product, change the amount and press **Add to cart**. The status text changes. The page does not load again.

An action can also answer with `c.SetSignals`, `c.Redirect` or `c.Toast`. An action that returns `nil` answers 204.

Next: [a form](/tutorial/a-form/).
