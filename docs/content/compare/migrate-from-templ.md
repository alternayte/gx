---
title: "Move from templ"
description: "How each templ construct maps to Gx, and the steps to move one component at a time."
section: Compare
order: 2
---

Gx and templ can live in one app. A Gx component is a Go function that returns a `gx.Node`, and `gx.Render` writes it in any handler. Move one component at a time.

## How the constructs map

| templ | Gx |
| --- | --- |
| `templ Card(title string)` in any `.templ` file | One file `Card.gx` with a `props` block. The file name is the component name. |
| Arguments by position | Props with names. A prop with no default is required. |
| `@Card("News")` | `<Card title="News" />` |
| `{ children... }` | A prop `Children gx.Node` and `{p.Children}` |
| `{ value }` | `{value}` |
| `if`, `for`, `switch` | The same. A statement line ends with `{`. |
| `templ.KV("active", ok)` in `class` | `class:active={ok}` |
| `templ.URL(s)` | A route value in `href`, or `gx.URL` |
| `templ.Raw(s)` | A value of type `gx.SafeHTML` |
| `hx-post="/cart/add"` | `on:click={route.Add{}}` with a `gx.Action` |
| `hx-target="#total"` | A fragment `#total` and `c.Patch(CartTotal(...))` |
| `templ generate` | `gx generate` |
| `templ fmt` | `gx fmt` |

## One component

This is a templ component.

```templ
package ui

templ Greeting(name string, loud bool) {
	<p class={ "greeting", templ.KV("font-bold", loud) }>
		Hello, { name }
		{ children... }
	</p>
}
```

This is the same component in Gx.

```gx title="ui/greeting/Greeting.gx"
package greeting

props {
  // Name is the person to greet.
  Name string
  // Loud makes the text bold.
  Loud bool = false
  // Children is the content after the greeting.
  Children gx.Node = nil
}

<p class="greeting" class:font-bold={p.Loud}>
  Hello, {p.Name}
  {p.Children}
</p>
```

A call names each prop.

```gx title="home/Home.gx"
package home

import "acme/ui/greeting"

<gx.Head title="Greeting" />
<greeting.Greeting name="Ada" loud={true}>
  <span>Welcome back.</span>
</greeting.Greeting>
<Counter label="Clicks" />
```

```text title="GET /"
Hello, Ada
<span>Welcome back.</span>
```

## Steps

1. Add Gx to the module: `go get github.com/alternayte/gx`. Add `cmd/gx/main.go` as a new app has it, or install the `gx` command.
2. Move one leaf component to a `.gx` file. Run `gx generate`.
3. Call it from a templ component as a Go function, or render it with `gx.Render(w, r, node)`.
4. Move a page after you move its components. Give each page a route type and a loader.
5. Change each `hx-` attribute to an action and a fragment when you want the typed loop. An app can keep htmx: release 0.2.0 has an htmx adapter. Read [Adapters](/guides/adapters/).

## Differences to know

- One component for each file. A `.templ` file with five components becomes five `.gx` files.
- A `.gx` file takes `if`, `for`, `switch` and `name := expr`. Put other Go code in a `.go` file of the same package.
- A component does not read the request context. Its loader passes the data as props.
- A URL in `href` is a route value. A string from data does not compile.
