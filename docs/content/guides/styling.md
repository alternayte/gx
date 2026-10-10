---
title: "Styling"
description: "Tailwind with no node, theme tokens, variants, class merge, icons and transitions."
section: Guides
order: 5
---

Gx uses Tailwind CSS v4 classes as you write them. It does not need node: `gx` downloads the Tailwind standalone binary for your computer, pins its version and its hash in `gx.lock`, and runs it.

## How the stylesheet is built

1. The compiler writes each class of the app to `.gx/classes.txt`. It reads the static `class` values, the `class:` directives, and the string literals in the Go files of a package that has `.gx` files.
2. `gx dev` and `gx build` run Tailwind over `app/theme.css` and that list.
3. The result is the Go file `gxstyles/styles_gx.go`. The app serves it at `/_gx/app.css`, and the binary holds it.

The class list also holds the value of a prop whose name ends in `Class`, such as `bodyClass` of `gx.Head`.

Tailwind sees only a class that is a full static string in the source. A class that the program makes later is the diagnostic [GX5003](/errors/GX5003/).

## A stylesheet for each route

`gx build` also makes a smaller stylesheet for each page route. A page then links a file with only the classes that the page can use, in the place of `/_gx/app.css`. You change no markup.

Add one line to `main.go`, after `gx.SetStylesheet`: `gx.SetRouteStylesheets(gxstyles.Routes())`. A new app from `gx init` has the line.

The compiler makes the class list of a route from these packages:

- The package that declares the page, the package of its view, and each package of your module that they import.
- The packages of each layout view, of each error view and of the toast of the app.

Keep each slice in its own package, and a page gets only the classes of its slice and of the components that it imports.

- A route has its own stylesheet only for `gx.Page(load, View)` with a `.gx` component as the view. Each other page links `/_gx/app.css`.
- Two routes with the same class list share one file. The name of the file has a hash of its content, so a browser keeps it.
- With layout-aware navigation, the runtime loads the stylesheet of the new page first. It then puts the new page in the document and removes the stylesheet of before.
- `gx dev` uses `/_gx/app.css` for each page.

Each distinct class list is one run of Tailwind. The key `route_sheets` in the `[styles]` table of `gx.toml` sets the largest number of lists; the default is 16. Above it, the build makes no route stylesheet and says so. A negative number turns the feature off.

## Theme tokens

`app/theme.css` holds the tokens in an `@theme` block. The token names are those of shadcn/ui: `--background`, `--foreground`, `--primary`, `--muted`, `--border`, `--radius` and the others. A theme from the shadcn/ui theme page works when you paste it.

Dark mode follows the `.dark` class and the system setting.

## Variants

A variant is a Go type with constants. `gx.Enum` is a map from each constant to a static class string. `gx lint` reports a constant with no entry as [GX5001](/errors/GX5001/).

`gx.Cx` joins class strings with the rules of tailwind-merge. A later class removes an earlier class that sets the same CSS property, so a caller can replace one class of a component.

```go title="ui/tag/styles.go"
// Package tag is a small label.
package tag

import "github.com/alternayte/gx"

// Tone is the colour of a tag.
type Tone string

// The tones of a tag.
const (
	Neutral Tone = "neutral"
	Success Tone = "success"
	Danger  Tone = "danger"
)

var toneClass = gx.Enum[Tone]{
	Neutral: "bg-muted text-foreground",
	Success: "bg-primary text-primary-foreground",
	Danger:  "bg-destructive text-white",
}

// class returns the classes of one tag. The class of the caller comes
// last, so it wins.
func (p TagProps) class() string {
	return gx.Cx("inline-flex items-center rounded-md px-2 py-0.5 text-xs font-medium", toneClass[p.Tone], p.Class)
}
```

```gx title="ui/tag/Tag.gx"
package tag

props {
  // Tone is the colour.
  Tone Tone = Neutral
  // Class adds classes to the element. A class here replaces a class of
  // the tag that sets the same property.
  Class string = ""
  // Children is the text.
  Children gx.Node
}

<span class={p.class()}>{p.Children}</span>
```

```gx title="home/Home.gx"
package home

import "acme/ui/tag"

<gx.Head title="Tags" />
<div class="flex gap-2">
  <tag.Tag>Draft</tag.Tag>
  <tag.Tag tone={tag.Success}>Live</tag.Tag>
  <tag.Tag tone={tag.Danger} class="px-4">Removed</tag.Tag>
</div>
<Counter label="Clicks" />
```

<Result page="guides/styling" get="/" />

<!-- expect GET /
bg-primary text-primary-foreground">Live
py-0.5 text-xs font-medium bg-destructive text-white px-4">Removed
-->

The third tag has `px-4` and no `px-2`: the class of the caller replaced it.

## Icons

```sh
gx icons pin lucide@1.0.0
```

`gx icons pin <set>@<version>` reads an Iconify pack and writes one `.gx` component for each icon to `ui/icons/<set>/`. An icon is an inline SVG that uses `currentColor`. It has `aria-hidden="true"`, and a `label` prop gives it `role="img"` and a name.

The binary holds only the icons that the app uses.

## View transitions

`gx.Transition[K](name)` makes a typed transition name. `transition={Hero(id)}` on an element in two pages joins the two elements, and the browser moves one into the other.

```go title="home/transition.go"
package home

import "github.com/alternayte/gx"

// Hero joins the picture of an item in a list and on its page.
var Hero = gx.Transition[int]("hero")
```

```gx title="home/Picture.gx"
package home

props {
  // ItemID is the item of the picture.
  ItemID int
}

<img transition={Hero(p.ItemID)} src="/pictures/item.png" alt="Item" width="80" height="80" />
```

- Two elements with one name in one template are the diagnostic [GX5002](/errors/GX5002/).
- Navigation between pages with a shared layout runs in a view transition.
- An action runs its patch in a transition with `c.Patch(gx.ViewTransition, nodes...)`.
- A reader who asks for reduced motion gets no transition.

## Offline and mirrors

`gx vendor` stores the pinned downloads in `.gx/vendor`. A build then needs no network.

A `[mirrors]` table in `gx.toml` gives a different address for each download. Each download also follows `HTTPS_PROXY`.
