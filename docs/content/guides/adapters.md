---
title: "Adapters: Datastar and htmx"
description: "The two hypermedia adapters, what each one has, and how to change an app to htmx."
section: Guides
order: 13
---

An adapter connects Gx to one hypermedia library in the browser. An app has one adapter. Gx has two: Datastar and htmx.

The htmx adapter arrives with release 0.2.0.

## What each adapter has

| Feature | Datastar | htmx |
| --- | --- | --- |
| Pages, layouts and typed links | Yes | Yes |
| Navigation inside a layout | Yes | Yes |
| Actions that patch fragments | Yes | Yes |
| Redirects and toasts | Yes | Yes |
| Forms and live validation | Yes | Yes |
| Registry components | Yes | Yes |
| Islands | Yes | Yes |
| Signals and client expressions | Yes | No |
| A strict Content Security Policy with no `'unsafe-eval'` | No | Yes |

Use Datastar when the app needs state in the browser: a signal, `show`, `text`, `bind:` or a signal statement in an `on:` handler. Use htmx when the server holds all the state, or when the app needs the strict policy.

## Choose the adapter

`gx init` asks for the adapter. The flag gives the answer:

```sh
gx init --adapter htmx acme
```

The adapter has two places in an app, and they must agree:

- `gx.toml` has the `adapter` key. The compiler reads it.
- `cmd/app/main.go` gives the adapter to `gx.Config`. The running app uses it.

When the two do not agree and a page declares a signal, the app answers with an error that names `gx.toml`.

## Actions are the same

An `on:` handler that is one route value works with each adapter. The generated Go is the same. The adapter writes its own attributes when the page renders.

| In the `.gx` file | Datastar | htmx |
| --- | --- | --- |
| `on:click={route.Add{}}` | `data-on:click="@post('/add')"` | `hx-post="/add" hx-trigger="click"` |
| `on:click.once` | `data-on:click__once` | `hx-trigger="click once"` |
| `on:click.stop` | `data-on:click__stop` | `hx-trigger="click consume"` |
| `on:keydown.window` | `data-on:keydown__window` | `hx-trigger="keydown from:window"` |
| `on:input.debounce(300ms)` | `data-on:input__debounce.300ms` | `hx-trigger="input delay:300ms"` |
| `on:scroll.throttle(1s)` | `data-on:scroll__throttle.1s` | `hx-trigger="scroll throttle:1s"` |
| `on:load` | `data-init` | `hx-trigger="load"` |
| `on:visible` | `data-on-intersect` | `hx-trigger="intersect"` |
| `on:interval(5s)` | `data-on-interval__duration.5s` | `hx-trigger="every 5s"` |

The handler is the same Go code. `c.Patch`, `c.Redirect` and `c.Toast` work with each adapter. Each patch mode works too: morph, `gx.Append`, `gx.Prepend`, `gx.Replace` and `gx.Remove`.

## What htmx does not have

htmx has no signals. Under the htmx adapter, each of these is the compile error [GX4006](/errors/GX4006/):

- A `signals` block.
- The `show`, `text`, `bind:`, `class:` and `attr:` directives with a signal.
- A signal statement in an `on:` handler, for example `on:click={$Open = !$Open}`.
- An action input field with a `signal` tag.
- The `.prevent` and `.outside` event modifiers. htmx stops the default action of a form and of a link itself.

`c.SetSignals` answers with an error.

Keep the state on the server and patch a fragment. For state that must live in the browser, use an [island](/guides/islands/).

## Change an app to htmx

The app of `gx init` with Datastar has one signal: the count of the counter. These steps move the count to the server.

1. Name the adapter at the top of `gx.toml`.

```toml title="gx.toml"
adapter = "htmx"
```

2. Give the htmx adapter to `gx.Config`.

```go title="cmd/app/main.go"
// Command app serves the acme app.
package main

import (
	"log"
	"net/http"
	"os"

	"acme/app"
	"acme/gxislands"
	"acme/gxstyles"
	"acme/home"

	"github.com/alternayte/gx"
	"github.com/alternayte/gx/adapters/htmx"
)

func main() {
	setupGallery()
	gx.SetStylesheet(gxstyles.CSS())
	gx.SetIslands(gxislands.Bundle())
	server := gx.New(gx.Config{Adapter: htmx.Adapter()})
	server.Group("/", app.Layout, gx.Nav(gx.MorphNavigation), home.Routes)

	// gx dev sets GX_DEV_ADDR.
	addr := os.Getenv("GX_DEV_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	log.Printf("acme listens on http://%s", addr)
	log.Fatal(http.ListenAndServe(addr, server))
}
```

3. Make the action a plain route. It reads no signal.

```go title="home/route/route.go"
// Package route holds the route types of the home slice.
package route

import "github.com/alternayte/gx"

// Home is the home page.
type Home struct {
	gx.Route `GET /{$}`
}

// Add is the action behind the Add one button of the counter.
type Add struct {
	gx.Route `POST /add`
}
```

4. Keep the count on the server. The action patches the count fragment.

```go title="home/home.go"
// Package home is the home slice: its page, its action and their views.
package home

import (
	"sync/atomic"

	"acme/home/route"

	"github.com/alternayte/gx"
)

// clicks is the state of the example. A real app keeps its state in a store.
var clicks atomic.Int64

// Page is the home page. The loader returns the props of the view.
var Page = gx.Page(func(c *gx.Ctx, in route.Home) (HomeProps, error) {
	return HomeProps{Clicks: int(clicks.Load())}, nil
}, Home)

// Add counts one click and patches the count of the counter.
var Add = gx.Action(func(c *gx.Ctx, in route.Add) error {
	return c.Patch(CounterCount(int(clicks.Add(1))))
})

// Routes lists every page, action and form of the slice.
var Routes = gx.Collect(Page, Add)
```

5. Remove the signal from the templates. The count is a prop and a fragment.

```gx title="home/Home.gx"
package home

props {
  // Clicks is the count that the server holds.
  Clicks int
}

<gx.Head title="Acme" />
<h1 class="text-3xl font-semibold">Welcome to acme</h1>
<Counter label="Clicks" clicks={p.Clicks} />
```

```gx title="home/Counter.gx"
package home

import "acme/home/route"

props {
  // Label names the counter.
  Label string
  // Clicks is the count that the server holds.
  Clicks int
}

<section class="mt-6 rounded-xl border border-border p-4">
  count := p.Clicks
  <p>{p.Label}: <span #count(count int)>{count}</span></p>
  <button class="mt-3 rounded-md bg-primary px-3 py-1.5 text-sm text-primary-foreground" on:click={route.Add{}}>Add one</button>
</section>
```

The page now has the attributes and the scripts of htmx:

```html title="GET /"
hx-post="/add" hx-trigger="click"
/_gx/htmx.js
```

## Your own htmx attributes

An `hx-` attribute in a `.gx` file is a plain attribute. Gx does not check it. Use an action and a fragment when you want the typed loop: the compiler then checks the route, the input and the target.

An answer of a Gx action has no main content. Each patch is an out-of-band swap, and the `HX-Reswap: none` header stops a swap into the element of the request.

## The files in the browser

Each adapter embeds a pinned file of its library in the binary. The app serves it below `/_gx/`. A page with no action, no form and no navigation loads no adapter script.

| Adapter | Files |
| --- | --- |
| Datastar | Datastar 1.0.4 |
| htmx | htmx 2.0.11, Idiomorph 0.8.0 and a small Gx file |

htmx has no morph of its own. The default patch mode of Gx is a morph, so the htmx adapter uses Idiomorph, the morph library of the htmx project.
