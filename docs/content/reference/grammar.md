---
title: "The .gx grammar"
description: "Every construct that the .gx parser accepts."
section: Reference
order: 3
---

This page lists every construct that the `.gx` parser accepts. Nothing
else parses. A construct that is not here is a parse error, `GX1000`.

## File

A file is one component. The shape is:

```text
package clause
import lines (optional)
props block (optional)
signals block (optional)
body
```

The component name is the file base name. It must be an exported Go
identifier. A file named `card.gx` gives `GX1001`. The package clause names the
Go package, for example `package card`.

An import is a Go import line or block: `import "path"` or `import ( ... )`.

## Blocks

`props` and `signals` blocks hold Go struct field syntax:

```text
props {
  // Title is the heading text.
  Title   string
  Variant Variant = Default
  Attrs   gx.Attrs = nil
}
```

- A field is `Name Type`. A field with `= expr` is optional.
- A default may span lines inside brackets or braces.
- A `//` comment on the lines above a field is the description of the field.
  The generated props struct carries it as a Go doc comment.
- A block holds no other comment. A comment after a field on its line, a
  comment with no field below it and a `/* ... */` comment give `GX1000`.
- Inside markup, `p` is the props value.
- `$Name` reads a signal. It is valid only in client expressions.

## Body nodes

The body holds these nodes, in any order:

- **element**: `<div>`, `<Button>`, `<ui.Card>`, `<wa-button>`, `<:slot>`.
- **text**: literal HTML text, including entities such as `&amp;`.
- **expression**: `{expr}`, a Go expression or a client expression.
- **control**: `if`, `for` and `switch` blocks.
- **statement**: `name := expr` on its own line.
- **comment**: `{/* ... */}` is stripped from the render; `<!-- ... -->`
  renders.
- **raw text**: the body of `script` and `style` is text, not markup.

## Elements

- A lowercase tag is an HTML element.
- An uppercase tag is a component in the same package.
- A dotted tag is a component from an imported package.
- A tag with a hyphen is a custom element.
- A `:` tag is a slot, such as `<:header>` or `<:row let={it}>`.
- A void element has no children and no closing tag: `br`, `hr`, `img`,
  `input`, `meta`, `link` and the other HTML void elements.
- A self-closing tag ends in `/>`.

## Attributes

An attribute is one of:

- **boolean**: `disabled`, `checked`, `open`
- **string**: `class="flex"` or `class='flex'`, and unquoted `class=flex`
- **expression**: `href={url}`, `class:shadow-lg={p.Variant == Raised}`
- **spread**: `{...p.Attrs}` on an HTML element
- **fragment**: `#row(it Item)` and `#name`, which set a component-scoped id
  and generate a typed fragment function

A directive name contains a colon: `class:name`, `bind:value`, `on:click`,
`attr:aria-expanded`, `show`, `text`.

## Control flow

A control line starts at a line start and ends with `{`:

```text
if p.Header != nil {
  <header>{p.Header}</header>
} else if p.Title != "" {
  ...
} else {
  ...
}

for _, it := range p.Items {
  <li>{it}</li>
}

switch p.Variant {
case Raised:
  <b>raised</b>
default:
  <b>flat</b>
}
```

Every Go `for` form is accepted: three-clause, `range` over a slice, map,
iterator or integer, and a bare `for`. A `case` holds one or more expressions
before `:`. Text that starts with `if`, `for` or `switch` but does not end
with `{` stays text.

A statement line is `name := expr`, or `a, b := expr`. The body of a block
ends at a `}` that is the first non-space character on its line.

## Expressions

`{expr}` in text or an attribute value is a Go expression, type-checked
against real Go types. Typical expressions are `p.Title`, `len(p.Items)` and
`$Qty > 0`. Strings and comments inside an expression may contain braces.

## Whitespace, comments and formatting

`gx fmt` has one canonical form:

- two-space indentation
- one attribute line, unless the source used one attribute per line
- one blank line between top-level sections
- Go expressions have single spaces between tokens
- a field description is `// text` on the lines directly above its field; a
  comment with no text is removed
- a whitespace-only gap between nodes stays a single space when the source gap
  had no newline, and becomes a newline plus indentation otherwise

`gx fmt` is idempotent and never changes rendered output.
