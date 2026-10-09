---
title: "4. A form"
description: "One rule set for the browser, the live validation and the server."
section: Tutorial
order: 4
sample: tutorial
---

This step adds an order form. The form input is a route type with rules.

## Make the form

```sh
gx new form shop/Order
```

The command adds two route types: `OrderPage` shows the form and `Order` receives it. It writes `shop/order_form.go` and `shop/OrderView.gx`.

## The rules

Add a `Name` field. The `Rules` method gives each field its rules. A rule points at a field with a pointer, so a rename changes the rule too.

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

// OrderPage is the page that shows the order form.
type OrderPage struct {
	gx.Route `GET /shop/order`
}

// Order is the input of the order form.
type Order struct {
	gx.Route `POST /shop/order`
	Name     string
	Email    string
}

// Rules holds the checks of the form. The browser, the live validation
// and the submit all use them.
func (in *Order) Rules() gx.Rules {
	return gx.Rules{
		gx.Field(&in.Name, gx.Required, gx.MaxLen(80)),
		gx.Field(&in.Email, gx.Required, gx.Email, gx.MaxLen(254)),
	}
}
```

## The handler

The handler runs only when each rule passes. `gx.FieldError` shows an error on one field for a check that needs data, such as a name that is in use.

```go title="shop/order_form.go"
package shop

import (
	"github.com/alternayte/gx"
	"acme/shop/route"
)

// Order receives the order form. The handler runs only when every rule
// passes.
var Order = gx.Form(func(c *gx.Ctx, in *route.Order) error {
	if in.Name == "nobody" {
		return gx.FieldError(&in.Name, "name.unknown")
	}
	return c.Redirect(route.Index{})
}, OrderView)

// OrderPage shows the empty form.
var OrderPage = gx.Page(func(c *gx.Ctx, in route.OrderPage) (OrderViewProps, error) {
	return Order.Props(&route.Order{}), nil
}, OrderView)
```

## The view

Gx generates the type `route.OrderForm` with one typed field for each input field. `Attrs()` gives the name, the id, the value and the browser constraints of a field.

`data-gx-validate="blur"` runs the rules of one field on the server when the reader leaves the field.

```gx title="shop/OrderView.gx"
package shop

import "acme/shop/route"

props {
  // F is the typed form: one field per input field.
  F route.OrderForm
}

<gx.Head title="Order" />
<h1 class="text-2xl font-semibold">Order</h1>
<form {...p.F.Attrs()} class="mt-4 grid max-w-sm gap-2">
  <label for={p.F.Name.ID}>Name</label>
  <input {...p.F.Name.Attrs()} type="text" class="rounded-md border border-border px-3 py-2" />
  <p id={p.F.Name.ID + "-error"} role="alert" class="text-sm text-destructive">{p.F.Name.Error}</p>
  <label for={p.F.Email.ID}>Email</label>
  <input {...p.F.Email.Attrs()} type="email" data-gx-validate="blur" class="rounded-md border border-border px-3 py-2" />
  <p id={p.F.Email.ID + "-error"} role="alert" class="text-sm text-destructive">{p.F.Email.Error}</p>
  <button type="submit" class="rounded-md bg-primary px-3 py-1.5 text-sm text-primary-foreground">Send</button>
</form>
```

<Result page="tutorial/a-form" get="/shop/order" />

<!-- expect GET /shop/order
data-gx-form="order"
name="email"
maxlength="254"
required
-->

The rules `Required` and `MaxLen` become the attributes `required` and `maxlength`. The browser checks them first.

## What happens on a submit

| Case | Answer |
| --- | --- |
| A rule fails, with JavaScript | The form element is patched with the values and the errors. |
| A rule fails, without JavaScript | Status 422 and the full page with the values and the errors. |
| Each rule passes | The handler runs. A redirect answers 303. |

A value of the wrong type, such as text in a number field, is a field error and not a 400.

Next: [components from the registry](/tutorial/registry-components/).
