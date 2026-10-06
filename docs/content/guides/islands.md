---
title: "Islands and web components"
description: "TypeScript islands with typed props, load strategies, npm packages with no node, and web components from npm as typed tags."
section: Guides
order: 12
---

An island is a TypeScript file that runs in the browser for one part of a page. The server renders the page. The island adds the behaviour that needs client code: a chart, an editor, a map.

Islands arrive with release 0.2.0.

## An island

A `.ts` file in a Go package is an island when its name starts with an upper-case letter and it has a default export. The file name is the component name. The props are the Go struct `<Name>Props` of the same package.

```go title="dashboard/charts.go"
package dashboard

// Point is one bar of the chart.
type Point struct {
	Label string `json:"label"`
	Value int    `json:"value"`
}

// RevenueChartProps are the props of the RevenueChart island.
type RevenueChartProps struct {
	Title string  `json:"title"`
	Data  []Point `json:"data"`
}
```

```ts title="dashboard/RevenueChart.ts"
import type { Mount, Update } from "./RevenueChart.props";

const mount: Mount = (el, { title, data }, ctx) => {
  const heading = el.appendChild(document.createElement("h2"));
  heading.textContent = title;
  const list = el.appendChild(document.createElement("ul"));
  for (const point of data) {
    list.appendChild(document.createElement("li")).textContent = `${point.label}: ${point.value}`;
  }
  // The cleanup runs when the island leaves the page.
  return () => el.replaceChildren();
};

export default mount;

// New props from the server arrive here. The island keeps its DOM.
export const update: Update = ({ title }, el) => {
  el.querySelector("h2")!.textContent = title;
};
```

A tag with the component name renders the island. An attribute sets a prop, as it does for a `.gx` component.

```gx title="dashboard/Board.gx"
package dashboard

props {
  Revenue []Point
}

<section>
  <RevenueChart title="Revenue" data={p.Revenue} load="eager" />
</section>
```

The server writes a `gx-island` element with the props as JSON:

```text title="GET /board"
<gx-island name="acme/dashboard/RevenueChart" props="{&#34;title&#34;:&#34;Revenue&#34;,&#34;data&#34;:[{&#34;label&#34;:&#34;Jan&#34;,&#34;value&#34;:10}]}" load="eager"
<div data-gx-island-root data-ignore-morph></div></gx-island>
```

```go title="dashboard/route/route.go"
// Package route holds the routes of the dashboard slice.
package route

import "github.com/alternayte/gx"

// Board is the page with the chart.
type Board struct {
	gx.Route `GET /board`
}
```

```go title="dashboard/dashboard.go"
package dashboard

import (
	"acme/dashboard/route"

	"github.com/alternayte/gx"
)

// BoardPage loads the numbers of the chart.
var BoardPage = gx.Page(func(c *gx.Ctx, in route.Board) (BoardProps, error) {
	return BoardProps{Revenue: []Point{{Label: "Jan", Value: 10}}}, nil
}, Board)

// Routes collects the routes of the slice.
var Routes = gx.Collect(BoardPage)
```

```go title="cmd/app/main.go"
// Command app runs the app.
package main

import (
	"log"
	"net/http"
	"os"

	"acme/app"
	"acme/dashboard"
	"acme/gxislands"
	"acme/gxstyles"
	"acme/home"

	"github.com/alternayte/gx"
	"github.com/alternayte/gx/adapters/datastar"
)

func main() {
	setupGallery()
	gx.SetStylesheet(gxstyles.CSS())
	gx.SetIslands(gxislands.Bundle())
	server := gx.New(gx.Config{Adapter: datastar.Adapter()})
	server.Group("/", app.Layout, gx.Nav(gx.MorphNavigation), home.Routes, dashboard.Routes)

	addr := os.Getenv("GX_DEV_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	log.Printf("listening on http://%s", addr)
	log.Fatal(http.ListenAndServe(addr, server))
}
```

## What the compiler writes

`gx generate` writes two files for each island, from the Go struct:

- `RevenueChart.props.ts` holds the TypeScript type `Props`, and the types `Mount`, `Update` and `Ctx`.
- `RevenueChart_gx.go` holds the Go function `RevenueChart`, which writes the props as JSON with no reflection.

The two files come from one mapping, so the type and the JSON cannot differ.

| Go | TypeScript |
| --- | --- |
| `string` | `string` |
| `int`, `float64` and the other numbers | `number` |
| `bool` | `boolean` |
| `time.Time` | `string` in RFC 3339 form |
| `[]T`, `[N]T` | `T[]` |
| `map[string]T` | `Record<string, T>` |
| a struct | an `interface` |
| a pointer field | an optional key |
| a string type with constants | a union of the values |
| `gx.SignalRef[T]` | `SignalRef<T>` |

A type with no mapping is [GX6002](/errors/GX6002/). A `gx.Secret` cannot be a prop. An island with no props struct is [GX6001](/errors/GX6001/).

## The mount function

The default export is `mount(el, props, ctx)`.

- `el` is the element that the island renders into.
- `props` has the type `Props`.
- `ctx.abort` is an `AbortSignal`. It ends when the island leaves the page.
- `ctx.signal(ref)` gives a signal of the page for a `gx.SignalRef` prop: `get`, `set` and `subscribe`.

The function can return a cleanup function. The optional `update(props, el, ctx)` export gets new props when a patch from the server changes them. An island with no `update` export mounts again with the new props.

## Morph safety

A patch or a navigation morphs the page. The morph does not touch the content that an island put into its element. It changes the props only.

A loop that holds an island needs a key: `<RevenueChart key={it.ID} ... />`. The state of each island then follows its row. A loop with no key is [GX2009](/errors/GX2009/).

## Load strategies

The `load` attribute names the time the browser loads the island file.

| Value | The browser loads the island |
| --- | --- |
| `load="eager"` | At once. |
| `load="idle"` | When the browser is idle. |
| `load="visible"` | When the element comes near the viewport. This is the default. |
| `load='media("(min-width: 768px)")'` | When the media query matches. |

A different value is [GX6004](/errors/GX6004/).

## The bundle

The `gx` command holds esbuild, so the build needs no node. `gx build` and `gx dev` bundle each island as one ES module. Code that two islands share goes into one common file. Each file name holds a hash of the content.

The result is the Go file `gxislands/islands_gx.go`. The app's main installs it with `gx.SetIslands(gxislands.Bundle())`, and the binary holds it. The app serves the files under `/_gx/islands/`.

A page loads the island loader only when it has an island.

## npm packages

`gx pin` stores the bundled ES module of an npm package in `js/vendor`, with each module that it imports.

```sh
gx pin d3-scale@4.0.2
```

An island then imports the package by its name: `import { scaleLinear } from "d3-scale"`. `gx.lock` records the hash of each file, and a changed file stops the build. A pinned package has the type `any` in TypeScript.

An app with a `package.json` and a `node_modules` directory does not use the pins. The bundler then finds each package as node does.

## The type check

`gx check` type-checks each island with the TypeScript compiler. The compiler is native code. `gx` downloads it one time and pins it in `gx.lock`, as it does for Tailwind. A TypeScript error is [GX6005](/errors/GX6005/).

The check uses strict options. An app with a `tsconfig.json` is checked as that project.

## Web components from npm

`gx wc pin` reads the custom elements manifest of an npm package. It writes a Go package with one typed tag for each element, and it pins the module of each element.

```sh
gx wc pin --element sl-badge --element sl-details @shoelace-style/shoelace@2.20.1
```

The tags of this example are in the package `ui/sl`. A page imports the package and writes `<sl.Badge variant="primary" pill>New</sl.Badge>`. The tag renders the element `sl-badge`.

The compiler checks the tag against the manifest:

- An attribute that the element does not have is [GX2003](/errors/GX2003/).
- An attribute with a fixed set of values takes a static string, and a value outside the set is [GX2004](/errors/GX2004/).
- A boolean attribute takes no value or a `bool` expression.
- `on:` takes each DOM event and each custom event that the element sends.
- `<:name>` fills a named slot of the element.

The other parts of a tag work as they do on an HTML element: `class`, a spread of `gx.Attrs`, and the client directives.

A morph keeps the element. It also keeps each attribute that the tag does not set, so an element that opened itself stays open.
