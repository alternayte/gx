---
title: "Components"
description: "The .gx format: props, expressions, control flow, attributes, slots and fragments."
section: Guides
order: 1
---

A `.gx` file is one component. The file name is the component name, so `Card.gx` is the component `Card`. The name starts with an upper-case letter.

A file has a package clause, optional Go imports, a `props` block, an optional `signals` block, and then HTML. Any valid HTML is valid `.gx`. The logic is Go.

Each `Card.gx` has a generated `Card_gx.go`. Commit the generated file. `go build` then works with no Gx command.

## Props

The `props` block uses the syntax of Go struct fields. A prop with `= value` is optional. A prop with no default is required: a call that omits it is the diagnostic [GX2001](/errors/GX2001/).

Inside the markup, `p` is the props value. A `//` comment above a prop is its description. The docs and `gx describe` show it.

```go title="ui/card/card.go"
// Package card is a card with a title, a footer and a list.
package card

import "github.com/alternayte/gx"

// Tone is the colour of a card.
type Tone string

// The tones of a card.
const (
	Plain Tone = "plain"
	Warn  Tone = "warn"
)

// DefaultRow renders one list item as text.
func DefaultRow(item string) gx.Node {
	return gx.Text(item)
}
```

```gx title="ui/card/Card.gx"
package card

props {
  // Title is the heading. It has no default, so it is required.
  Title string
  // Tone is the colour. It has a default, so it is optional.
  Tone Tone = Plain
  // Footer is a named slot.
  Footer gx.Node = nil
  // Children is the default slot: the content of the tag.
  Children gx.Node
  // Attrs adds HTML attributes to the root element.
  Attrs gx.Attrs = nil
}

<article class="rounded-xl border border-border p-4" class:border-destructive={p.Tone == Warn} {...p.Attrs}>
  <h3 class="font-semibold">{p.Title}</h3>
  {p.Children}
  if p.Footer != nil {
    <footer class="mt-3 text-sm text-muted-foreground">{p.Footer}</footer>
  }
  {/* A comment in braces is not in the output. */}
</article>
```

## Expressions

`{expr}` in text or in an attribute value is a Go expression. The Go type checker checks it against the real types. An error has the line and the column of the `.gx` file.

Gx escapes each value for its place: text, attribute, URL or style. Raw HTML needs the type `gx.SafeHTML`.

## Control flow

Control flow is Go: `if`, `else if`, `else`, each form of `for`, and `switch`. A statement line starts with the keyword and ends with `{`. A line `name := expr` declares a local.

A loop that holds an input, a component with signals or a transition needs a key. Read [GX2009](/errors/GX2009/).

## Attributes

| Form | Meaning |
| --- | --- |
| `class="a b"` | Static classes. |
| `class:name={cond}` | Adds the class when `cond` is true. |
| `disabled={cond}` | A boolean attribute. It is not in the output when `cond` is false. |
| `{...expr}` | Spreads a `gx.Attrs` value on an HTML element. |
| `href={route.Show{ID: 1}}` | A typed link. |

A component gets no attribute that it does not declare. To pass HTML attributes through, declare a prop of type `gx.Attrs` and spread it, as `Card` does.

## Slots

A slot is a prop.

- `Children gx.Node` takes the content of the tag.
- A named prop of type `gx.Node` takes `<:name>...</:name>`.
- A prop of type `gx.Slot[T]` takes `<:name let={v}>...</:name>`. The component calls it as `{p.Name(v)}`.

```gx title="ui/card/List.gx"
package card

props {
  // Items is the list of names.
  Items []string
  // Row renders one item. A caller can replace it.
  Row gx.Slot[string] = DefaultRow
}

<ul class="mt-2 list-disc pl-5">
  for _, item := range p.Items {
    <li>{p.Row(item)}</li>
  }
</ul>
```

## Use a component

An upper-case tag is a component of the same package. A tag `pkg.Name` is a component of an imported package. An attribute name is the prop name with a lower-case first letter.

```gx title="home/Home.gx"
package home

import "acme/ui/card"

props {
  // Names is the list of products that are low.
  Names []string = nil
}

<gx.Head title="Components" />
<card.Card title="Stock" tone={card.Warn}>
  <:footer>Updated today</:footer>
  switch len(p.Names) {
  case 0:
    <p>No product is low.</p>
  default:
    <p>These products are low.</p>
    <card.List items={p.Names}>
      <:row let={name}><strong>{name}</strong></:row>
    </card.List>
  }
</card.Card>
```

```go title="home/home.go"
// Package home is the home slice: its page, its action and their views.
package home

import (
	"github.com/alternayte/gx"
	"acme/home/route"
)

// Page is the home page. The loader returns the props of the view.
var Page = gx.Page(func(c *gx.Ctx, in route.Home) (HomeProps, error) {
	return HomeProps{Names: []string{"Tea", "Coffee"}}, nil
}, Home)

// Save patches the saved text of the counter that invoked it.
var Save = gx.Action(func(c *gx.Ctx, in route.Save) error {
	key := gx.ScopeKey(gx.Scope(c.R), "home.Counter")
	return c.Patch(CounterSaved(key, in.Count))
})

// Routes lists every page, action and form of the slice.
var Routes = gx.Collect(Page, Save)
```

```text title="GET /"
<h3 class="font-semibold">Stock</h3>
<strong>Tea</strong>
Updated today
border-destructive
```

## Fragments

`#name` or `#name(params)` marks an element as a fragment. The element gets an id, and Gx generates a function that renders the element alone, for example `CounterSaved`. An action sends the result of that function as a patch.

A fragment can use its parameters and `p` only. A different local is the diagnostic [GX2008](/errors/GX2008/).

## Format

```sh
go run ./cmd/gx fmt home/Home.gx
go run ./cmd/gx fmt --check home/Home.gx
```

`gx fmt` gives one form for each file. It formats the Go parts as `gofmt` does. It does not change the output of the component.

## Rules that keep templates simple

- A component renders from its props only. It does no IO. A loader does the IO.
- A component does not change its props.
- No component is global. A Go import brings each one in.
