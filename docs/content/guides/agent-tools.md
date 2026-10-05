---
title: "Tools for coding agents"
description: "The app model as JSON, machine diagnostics, fixtures, the dev MCP server and AGENTS.md."
section: Guides
order: 7
---

A coding agent reads and changes a Gx app through the same typed model that the compiler uses. Each tool here also helps a person.

## The app model

```sh
go run ./cmd/gx describe --json
go run ./cmd/gx describe --schema
```

`gx describe --json` prints one object.

| Key | Content |
| --- | --- |
| `components` | Each component with its props (type, default, required, description), signals, fragments and fixture names. |
| `routes` | Each route type with its fields, its handler and its mount. |
| `actions` | Each action with the signals that it reads. |
| `forms` | Each form with its fields and the rules of each field. |
| `transitions`, `icons`, `registry` | The transition names, the pinned icon sets and the installed registry items. |

`gx describe --schema` prints the JSON Schema of this object.

## Diagnostics for a tool

```sh
go run ./cmd/gx check --json
go run ./cmd/gx lint --json
```

Each diagnostic has a code, a file, a line, a column, a message, a link to its page and, where Gx knows one, a fix. The [diagnostics index](/reference/diagnostics/) lists each code.

## Fixtures and the gallery

A file `<Name>.fixtures.go` declares named prop sets for a component. The dev gallery at `/_gx/gallery` renders each fixture. A component with no fixtures shows as missing.

```go title="home/Counter.fixtures.go"
package home

import "github.com/alternayte/gx"

// CounterFixtures are the examples of Counter in the dev gallery.
var CounterFixtures = gx.Fixtures[CounterProps]{
	"Default":   {Label: "Clicks"},
	"LongLabel": {Label: "The number of times that you pressed the button"},
}
```

The gallery is in a dev build only. A production binary has no `/_gx/gallery` route.

## The dev MCP server

```sh
go run ./cmd/gx mcp
```

`gx mcp` serves the Model Context Protocol on standard input and output. Add the command to the MCP settings of your agent.

| Tool | What it does |
| --- | --- |
| `describe` | Returns the app model. |
| `check` | Returns the diagnostics of `gx check`. |
| `routes` | Returns the route list. |
| `render_fixture` | Renders one fixture as HTML or as a PNG screenshot. |
| `screenshot_route` | Returns a PNG screenshot of one page. |
| `a11y_audit` | Audits a page or a fixture with axe-core and returns the violations. |
| `registry_search` | Finds registry items by name, kind or description. |
| `registry_add` | Installs a registry item, as `gx add` does. |

The screenshot and audit tools build the app, run it, and open it in headless Chrome. They need Chrome or Chromium. They do not need node.

## AGENTS.md

`gx init` writes `AGENTS.md` with the commands, the conventions and the rules for app code.

```sh
go run ./cmd/gx agents --update
```

`gx agents --update` writes the current text into the managed section. The section is between two marker comments. The text that you write outside the section stays.
