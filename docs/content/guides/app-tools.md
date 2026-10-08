---
title: "Actions as tools for agents"
description: "An action or a form with .Tool() is a tool that the agent of a user can call: in the browser through WebMCP, and on the server through MCP with package gxmcp."
section: Guides
order: 16
---

The agent of a user can call the actions of your app. An action or a form with `.Tool()` is a tool. One definition gives the tool its name, its description, its input schema and its handler.

Tools arrive with release 0.3.0. This guide is about the tools of your app. For the tools that help a coding agent change the app, read [Tools for coding agents](/guides/agent-tools/).

## A tool

Add `.Tool()` to an action. Write a doc comment above the variable of the action: it is the description that the agent reads.

```go title="notes/route/route.go"
// Package route holds the routes of the notes slice.
package route

import "github.com/alternayte/gx"

// Add adds a note to a list.
type Add struct {
	gx.Route `POST /lists/{list}/notes`
	List     string
	Text     string
	Color    string
	Pin      bool `query:"pin"`
}

func (in *Add) Rules() gx.Rules {
	return gx.Rules{
		gx.Field(&in.Text, gx.Required, gx.MaxLen(280)),
		gx.Field(&in.Color, gx.OneOf("yellow", "blue", "green")),
	}
}

// Clear removes each note of a list.
type Clear struct {
	gx.Route `POST /lists/{list}/clear`
	List     string
}
```

```go title="notes/notes.go"
package notes

import (
	"acme/notes/route"

	"github.com/alternayte/gx"
)

// Added is the result of the tool for the agent.
type Added struct {
	List  string `json:"list"`
	Count int    `json:"count"`
}

// Adds a note to a list of the user. The text has at most 280 characters.
var add = gx.Action(func(c *gx.Ctx, in route.Add) error {
	gx.ToolResult(c, Added{List: in.List, Count: 1})
	return c.Toast("Note added")
}).Tool()

// Removes each note of a list. The user cannot undo this.
var clear = gx.Action(func(c *gx.Ctx, in route.Clear) error {
	return c.Toast("List cleared")
}).Tool(gx.Confirm)

// Routes collects the routes of the slice.
var Routes = gx.Collect(add, clear)
```

- The name of the tool comes from the slice and the type: `notes_add` and `notes_clear`.
- The description is the doc comment. A tool with no doc comment is [GX4011](/errors/GX4011/).
- The arguments are the fields of the input: the path variable `list`, the query value `pin`, and the form fields `text` and `color`.
- `gx.Confirm` makes the browser ask the user before an agent runs the tool. Use it for an action that the user cannot undo.

Only an action or a form with `.Tool()` is a tool. A tool call runs the handler of its tool: no argument of the agent selects a different route.

## The input schema

The compiler writes the JSON Schema of the arguments from the input struct and its `Rules()`. The rules that check a form also tell the agent what a correct call is.

| Rule | JSON Schema |
| --- | --- |
| `gx.Required` | The field is in `required`. |
| `gx.Email` | `format: email` |
| `gx.IsURL` | `format: uri` |
| `gx.MinLen(n)`, `gx.MaxLen(n)` | `minLength`, `maxLength` |
| `gx.Min(n)`, `gx.Max(n)` | `minimum`, `maximum` |
| `gx.Pattern(re)` | `pattern` |
| `gx.OneOf(values...)` | `enum` |
| `gx.True(key)` | `const: true` |
| `gx.Each(rule)` | The keywords of `rule` in `items`. |
| `gx.Check`, `gx.CheckCtx` | None. The rule runs on the server. |

A path variable is always in `required`. A `default` tag is the `default` of the field. A struct field is an object and a slice is an array.

The schema is a help for the agent and not the check. Each call runs the binder and the rules on the server, as a request of a browser does.

## The answer of a tool

`gx.ToolResult(c, v)` sets the structured output of the call. The agent gets `v` as JSON. For a request of a browser it does nothing, so one handler serves the page and the agent.

An action with no `gx.ToolResult` answers with a summary of its patches, one line for each patch: the element that it changed, the signals that it set, the toast or the redirect.

A call that breaks a rule is an error answer with the field and the message key. An error of the handler is an error answer with its text.

## Tools in the browser

A browser with WebMCP gives the agent of the user the tools of the page. Gx registers a tool while an element that invokes it is in the page: a button with `on:click={route.Add{...}}`, or the form of a form tool. When the element leaves the page, the tool leaves the browser. You write no code for this.

```gx title="notes/Board.gx"
package notes

import "acme/notes/route"

props {
  List string
}

<section>
  <h2>{p.List}</h2>
  <button on:click={route.Add{List: p.List, Text: "A new note"}}>Add a note</button>
  <button on:click={route.Clear{List: p.List}}>Clear the list</button>
</section>
```

- A call of the tool runs the action on the server with the arguments of the agent. The answer changes the page as it does after a click, and the agent gets the result.
- A tool with `gx.Confirm` asks the user first, with a dialog of the browser.
- The runtime uses `document.modelContext`. It falls back to `navigator.modelContext`, which an earlier draft of WebMCP used. A browser with neither loads the page with no change.
- A page with no tool element loads no script for tools.

WebMCP is a draft of a W3C Community Group, and its API can change. The tool module of the Gx runtime is the one place that knows the API. A change of the draft needs a new Gx and no change of your app.

## Serve the tools over MCP

Package `gxmcp` serves the tools over the Model Context Protocol with streamable HTTP. It is a package of its own because it uses the official MCP Go SDK. An app that does not import it does not compile the SDK.

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
	"acme/notes"

	"github.com/alternayte/gx"
	"github.com/alternayte/gx/adapters/datastar"
	"github.com/alternayte/gx/gxmcp"
)

// signedIn lets a request with a session pass. An app has its own auth.
func signedIn(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := r.Cookie("session"); err != nil && r.Header.Get("Authorization") == "" {
			http.Error(w, "sign in first", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	setupGallery()
	gx.SetStylesheet(gxstyles.CSS())
	gx.SetWidgetStylesheets(gxstyles.Widgets())
	gx.SetIslands(gxislands.Bundle())
	server := gx.New(gx.Config{Adapter: datastar.Adapter()})
	server.Group("/", app.Layout, gx.Nav(gx.MorphNavigation), home.Routes)
	server.Group("/", signedIn, notes.Routes)
	// The tools of the app, behind the auth of the app.
	gxmcp.Mount(server, "/mcp", signedIn)

	// gx dev sets GX_DEV_ADDR.
	addr := os.Getenv("GX_DEV_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	log.Printf("acme listens on http://%s", addr)
	log.Fatal(http.ListenAndServe(addr, server))
}
```

The middleware of `gxmcp.Mount` runs before each MCP request. A request with no auth gets no tool list and no call.

## The security rules of a tool call

A tool call is not a second way into the app. It runs the request of its action through the routes of the app, with the headers of the MCP request.

- The middleware of the group of the action runs for the call. A user that the group refuses cannot run the tool.
- The cross-origin check of the app applies to the MCP endpoint and to the tool routes of a page. A page of a different site cannot call a tool with the cookies of the user.
- The binder fills only the declared fields, and the rules run before the handler.
- A value of type `gx.Secret` in a tool result is [GX7002](/errors/GX7002/). A secret that the compiler cannot see is `[redacted]` in the JSON. A secret as the key of a map has no redacted form, so the agent gets an error answer and no result.
