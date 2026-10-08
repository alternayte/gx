---
title: "Widgets"
description: "A component of a Gx app as a custom element on a page of a different site: the server renders it, and the host page needs one script tag."
section: Guides
order: 14
---

A widget is a component that a page of a different site uses as a custom element. The different site is the host. The host is not a Gx app: it can be a React app or a plain HTML page.

Your Gx server renders the widget and answers its actions. The element on the host page is a small loader. It gets the HTML from your server and puts it into its shadow root. A deploy of your server changes the widget on each host, and the host changes nothing.

Widgets arrive with release 0.3.0.

## A widget

A widget is a GET route, a loader and a view, as a page is. The component has nothing in it that is specific to a widget, so one component serves a page of your app and a widget.

The fields of the route with a `query` tag are the attributes of the element. A field is a string, a number or a bool.

```go title="cart/route/route.go"
package route

import "github.com/alternayte/gx"

// Widget is the cart as a widget. Its fields are the attributes of the
// <acme-cart> element.
type Widget struct {
	gx.Route `GET /cart`
	Currency string `query:"currency" default:"EUR"`
	Compact  bool   `query:"compact"`
}

// Add adds the quantity of the widget to the cart.
type Add struct {
	gx.Route `POST /cart/add`
	Qty      int `signal:"qty"`
}

func (in *Add) Rules() gx.Rules {
	return gx.Rules{gx.Field(&in.Qty, gx.Min(1), gx.Max(99))}
}
```

```gx title="cart/Cart.gx"
package cart

import "acme/cart/route"

props {
  Currency string
  Compact  bool
  Count    int
}

signals {
  Qty int = 1
}

<section class="rounded-xl border border-border bg-card p-4 text-card-foreground">
  count := p.Count
  <p><span #count(count int)>{count}</span> items, prices in {p.Currency}</p>
  if !p.Compact {
    <input type="number" min="1" max="99" bind:value={$Qty} />
  }
  <button class="rounded-md bg-primary px-3 py-1.5 text-primary-foreground" on:click={route.Add{}}>Add</button>
</section>
```

```go title="cart/cart.go"
package cart

import (
	"acme/cart/route"

	"github.com/alternayte/gx"
)

// Base is the signal namespace of one Cart instance.
const Base = "cart.Cart"

// ChangedDetail is the detail of the cart-changed event.
type ChangedDetail struct {
	Count int `json:"count"`
}

// Changed tells the host page that the cart changed.
var Changed = gx.Event[ChangedDetail]("cart-changed")

// Widget is the cart as the <acme-cart> element.
var Widget = gx.Widget(func(c *gx.Ctx, in route.Widget) (CartProps, error) {
	return CartProps{Currency: in.Currency, Compact: in.Compact, Count: 2}, nil
}, Cart).Tag("acme-cart")

var add = gx.Action(func(c *gx.Ctx, in route.Add) error {
	count := 2 + in.Qty
	c.Emit(Changed(ChangedDetail{Count: count}))
	return c.Patch(CartCount(gx.ScopeKey(gx.Scope(c.R), Base), count))
})

// Routes collects the widget and each action that its component invokes.
var Routes = gx.Collect(Widget, add)
```

The loader makes the props. The host sends only the attributes, and the attributes pass the binder and the `Rules()` of the route before the loader reads them. A value that does not convert gives the host an error with status 400.

## Mount the widget

A host page is a different origin, so the group of the widget lists the origins that can call it. `gx check` reports a widget, or an action of its component, in a group with no origin list (GX6008).

```go title="cmd/app/main.go"
// Command app serves the acme app.
package main

import (
	"log"
	"net/http"
	"os"

	"acme/app"
	"acme/cart"
	"acme/gxislands"
	"acme/gxstyles"
	"acme/home"

	"github.com/alternayte/gx"
	"github.com/alternayte/gx/adapters/datastar"
)

func main() {
	setupGallery()
	gx.SetStylesheet(gxstyles.CSS())
	gx.SetWidgetStylesheets(gxstyles.Widgets())
	gx.SetIslands(gxislands.Bundle())
	server := gx.New(gx.Config{Adapter: datastar.Adapter()})
	server.Group("/", app.Layout, gx.Nav(gx.MorphNavigation), home.Routes)
	// The pages of these origins can show the widgets.
	server.Group("/widgets",
		gx.AllowOrigins("https://shop.example.com", "https://*.partner.io"),
		cart.Routes)

	// gx dev sets GX_DEV_ADDR.
	addr := os.Getenv("GX_DEV_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	log.Printf("acme listens on http://%s", addr)
	log.Fatal(http.ListenAndServe(addr, server))
}
```

An origin is exact, or `https://*.host` for the subdomains of a host, or `gx.AnyOrigin` for a public widget.

A request from an origin of `gx.AllowOrigins` carries no cookie of the user: the server removes the `Cookie` header. `gx.AllowCredentials` lists the exact origins that can send cookies. Use it only for a site that you own.

## Use the widget on a host page

Your server serves the element file of each widget at `/_gx/widgets/<tag>.js`. A host page needs one script tag and no package.

```html
<script type="module" src="https://api.acme.dev/_gx/widgets/acme-cart.js"></script>

<acme-cart currency="USD" compact>
  <p>Loading your cart…</p>
</acme-cart>
```

The file takes the address of your server from its own URL. The children of the tag are fallback content: they show while the widget loads and when it fails.

A changed attribute gets a new render from the server. The element morphs the new HTML into the old, so signals and typed input values stay.

## Lifecycle and events

The element has the attribute `data-gx-state` with the value `loading`, `ready` or `error`. It fires three events of its own, and each one bubbles out of the shadow root.

| Event | When | Detail |
| --- | --- | --- |
| `gx-ready` | One time, after the first HTML is in the shadow root. | None. |
| `gx-error` | The first load or a later action fails. | `status` and `key`. No text of the server. |
| `gx-navigate` | An action calls `c.Redirect`. The page does not move. | `url`. |

`gx.Event[D](name)` declares a typed event of your domain. `c.Emit` in an action sends it with the answer, and the element dispatches a `CustomEvent` with that name.

```html
<script type="module">
  const cart = document.querySelector("acme-cart");
  cart.addEventListener("cart-changed", (e) => console.log(e.detail.count));
  cart.addEventListener("gx-error", (e) => console.log(e.detail.status, e.detail.key));
</script>
```

## The user of the host

The element has a `token` property: a string, or a function that returns a string or a promise of one. The element sends it as `Authorization: Bearer <token>` on each request. After an answer with status 401 it calls the function one more time.

```html
<script type="module">
  document.querySelector("acme-cart").token = async () => (await fetch("/api/cart-token")).text();
</script>
```

The token is a property and never an attribute, so it is in no markup. Gx does not issue or check tokens. The middleware of your group reads the header.

## Styles

Each widget has its own stylesheet in its shadow root. The stylesheet holds only the classes that the widget uses. A rule of the host page does not select an element in the widget.

Two things of the host do reach the widget. An inherited property that the widget does not set, such as the text colour or the font, comes from the element of the host. A size in `rem` follows the font size of the `html` element of the host. Set the text colour and the font in your component when the widget must look the same on each host.

The tokens of your theme are CSS variables on the element, so the host can set them:

```html
<style>
  acme-cart { --primary: #0a58ca; --radius: 0.25rem; }
</style>
```

The default theme turns dark from the system preference. A host sets the class `light` or `dark` on the element to fix the mode.

## What is different in a widget

Signals, client expressions, actions, fragments, forms with live validation, uploads, registry components and islands work in a widget as on a page. These parts are different:

- `c.Toast` shows the toast in the shadow root.
- `c.Redirect` fires `gx-navigate`. The host owns its navigation.
- A typed link is an absolute URL of your server.
- `gx.Head` in the view of a widget is an error (GX6007). A widget has no document.
- Morph navigation, layouts and view transitions on patches do not apply.

The widget script evaluates each client expression as data. A host page with a strict Content Security Policy needs no `unsafe-eval` and no `unsafe-inline`. The policy must let the page load scripts from your server and connect to it.

## Try a widget in dev

`gx dev` serves a host page for each widget at `/_gx/widgets/<tag>`, and a list of the widgets at `/_gx/widgets`. The page loads the widget from the other loopback name of the dev server, which is a different origin for the browser. It has a field for each attribute, a field for the token and a log of the events.

## A package for the host

A host that wants types, or a pinned copy of the element file, gets an npm package. `gx.toml` names the package:

```toml
[widgets]
name = "@acme/widgets"
version = "1.0.0"
server = "https://api.acme.dev"
```

```sh
gx wc build
gx wc pack
NPM_TOKEN=... gx wc publish
```

`gx wc build` writes three files for each widget: the element file, a `.d.ts` file and a file with the JSX types for React. It writes one `custom-elements.json` for the app. `gx wc pack` puts them into an npm tarball. `gx wc publish` sends the tarball to the npm registry over HTTP. No command needs node.

The element file of a package calls the `server` of `gx.toml`. The file holds no runtime, so a host needs a new version of the package only when the contract changes.

## Keep the contract

The contract of a widget with its hosts has four parts: its attributes, its events, the fields of each event detail, and the CSS variables that its stylesheet reads. `gx wc check` compares the contract with the baseline. The baseline is the contract of the last published version, in `.gx/base/widgets.json`. Commit that file.

| Change | Version that `gx wc check` needs |
| --- | --- |
| A removed attribute, event, detail field or CSS variable | A major bump |
| A different type of an attribute, an event or a detail field | A major bump |
| A removed widget | A major bump |
| A new attribute, event, detail field, CSS variable or widget | A minor bump |

```sh
gx wc check
```

The command prints each change and fails when the `version` of `gx.toml` does not have the bump. Run it in CI. `gx wc publish` runs the same check first, and writes the baseline after the registry takes the version.

An app that publishes no package still has hosts. `gx wc check --update` records the contract of now as the baseline, after the check passes. Run it when you deploy a version.
