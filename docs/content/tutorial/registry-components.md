---
title: "5. Components from the registry"
description: "Copy a component into the app, use it and change it."
section: Tutorial
order: 5
sample: tutorial
---

The registry holds components with the names and the look of shadcn/ui. `gx add` copies the source into the app. You own the copy and can change it.

## Add the button

```sh
go run ./cmd/gx add button
```

The command writes `ui/button/` and records the item in `gx.lock`. It also stores a snapshot in `.gx/base/`, so `gx update` can merge a later release with your changes.

## Use the button

A tag with a package name is a component of that package. Import the package first.

`variant` is a typed prop. `variant="outline"` is the diagnostic [GX2004](/errors/GX2004/): a static string does not become a constant.

```gx title="shop/OrderView.gx"
package shop

import (
  "acme/shop/route"
  "acme/ui/button"
)

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
  <div class="flex gap-2">
    <button.Button type="submit">Send</button.Button>
    <button.Button type="reset" variant={button.Outline}>Clear</button.Button>
  </div>
</form>
```

```text title="GET /shop/order"
data-slot="button"
data-variant="outline"
```

## How the component makes its classes

Open `ui/button/styles.go`. The variants are a `gx.Enum` map from a constant to a static class string. `gx lint` reports a constant with no entry as [GX5001](/errors/GX5001/).

`gx.Cx` joins the class strings. A later class removes an earlier class that sets the same CSS property, so `class="px-8"` on a button replaces its `px-4`.

Each class is a full static string in the source. Tailwind reads the source files, and it cannot see a class that the program makes later. That is the diagnostic [GX5003](/errors/GX5003/).

## Keep a component up to date

```sh
go run ./cmd/gx diff button
go run ./cmd/gx update button
```

`gx diff` shows what changed in your copy and in the registry. `gx update` merges the two. A conflict gets conflict marks in the file, and the command reports it.

Next: [check, build and ship](/tutorial/ship/).
