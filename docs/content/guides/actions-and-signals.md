---
title: "Actions and signals"
description: "Server actions, typed patches, client signals and client expressions."
section: Guides
order: 3
---

An action is a route type and a handler. A template invokes it with a struct literal. The handler answers with typed patches.

A signal is a value in the browser. It belongs to one component instance.

```sh
gx new slice todo
```

## Routes of the example

```go title="todo/route/route.go"
// Package route holds the route types of the todo slice. A route
// package holds only route types.
package route

import "github.com/alternayte/gx"

// Index is the list page.
type Index struct {
	gx.Route `GET /todo`
}

// Add adds one item. Title comes from the Title signal of the page.
type Add struct {
	gx.Route `POST /todo/add`
	Title    string `signal:"title"`
}

// Rules checks the title. The browser controls each signal value.
func (in *Add) Rules() gx.Rules {
	return gx.Rules{gx.Field(&in.Title, gx.Required, gx.MaxLen(80))}
}

// Clear removes every item.
type Clear struct {
	gx.Route `POST /todo/clear`
}

// Count gives the number of items. The page asks for it when the count
// comes into view.
type Count struct {
	gx.Route `GET /todo/count`
}
```

## Signals and client expressions

The `signals` block declares each signal with a type and a first value. Inside the markup, `$Name` is the signal.

An attribute that reads a signal is a client expression. Gx changes it into the syntax of the adapter. Signals and client expressions need the Datastar adapter: read [Adapters](/guides/adapters/). A client expression is a small part of Go: literals, signals, values from the server, operators, and the functions of package `gxc`.

| Directive | Example | Meaning |
| --- | --- | --- |
| `show` | `show={$Open}` | Shows the element when the value is true. |
| `text` | `text={$Title}` | Sets the text of the element. |
| `bind:<prop>` | `bind:value={$Title}` | Joins an input and a signal in two directions. |
| `class:<name>` | `class:font-semibold={$Open}` | Adds or removes a class. |
| `attr:<name>` | `attr:aria-expanded={$Open}` | Sets an attribute. |
| `on:<event>` | `on:click={$Open = !$Open}` | Runs signal statements or invokes an action. |

```gx title="todo/IndexView.gx"
package todo

import "acme/todo/route"

props {
  // Items is the list at the first load.
  Items []string = nil
}

signals {
  // Title is the text of the new item.
  Title string = ""
  // Open shows the help text.
  Open bool = false
}

<gx.Head title="To do" />
<h1 class="text-2xl font-semibold">To do</h1>
<div class="mt-4 flex gap-2">
  <input type="text" bind:value={$Title} class="rounded-md border border-border px-3 py-2" />
  <button class="rounded-md bg-primary px-3 py-1.5 text-sm text-primary-foreground" on:click={route.Add{}}>Add</button>
  <button class="rounded-md border border-border px-3 py-1.5 text-sm" on:click={route.Clear{}}>Clear</button>
</div>
<p class="mt-2 text-sm text-muted-foreground">New item: <span text={$Title}></span></p>
items := p.Items
<ul #items(items []string) class="mt-4 list-disc pl-5">
  for _, item := range items {
    <li>{item}</li>
  }
</ul>
<button class="mt-4 text-sm underline" on:click={$Open = !$Open} attr:aria-expanded={$Open}>Help</button>
<p show={$Open} class:font-semibold={$Open}>Write a title and press Add.</p>
<p class="mt-4 text-sm" on:visible={route.Count{}}>
  <span id="todo-count">Counting</span>
</p>
```

## Answers of an action

| Answer | Effect |
| --- | --- |
| `c.Patch(nodes...)` | Sends fragments. The browser morphs each one into the element with the same id. |
| `c.Update(node)` | Takes a component from new data and sends only the fragments that changed. |
| `c.SetSignals(v)` | Sets the signals of the instance that invoked the action. |
| `c.Redirect(route)` | Goes to a page. |
| `c.Toast(text)` | Shows a toast in the toaster region. |
| `nil` with no call | Answers 204. The page does not change. |

Gx generates a function for each fragment and a `Signals` struct for each component with signals. Here they are `IndexViewItems` and `IndexViewSignals`.

A fragment of a component with signals takes the instance key as its first parameter. `gx.ScopeKey` reads the key from the request.

```go title="todo/todo.go"
// Package todo is the todo slice: its pages, actions and views.
package todo

import (
	"strconv"
	"sync"

	"github.com/alternayte/gx"
	"acme/todo/route"
)

// The items live in memory. A real app uses a database.
var (
	mu    sync.Mutex
	items []string
)

// IndexPage is the list page.
var IndexPage = gx.Page(func(c *gx.Ctx, in route.Index) (IndexViewProps, error) {
	mu.Lock()
	defer mu.Unlock()
	return IndexViewProps{Items: append([]string(nil), items...)}, nil
}, IndexView)

// Add adds one item, patches the list and empties the text field.
var Add = gx.Action(func(c *gx.Ctx, in route.Add) error {
	mu.Lock()
	items = append(items, in.Title)
	list := append([]string(nil), items...)
	mu.Unlock()
	key := gx.ScopeKey(gx.Scope(c.R), "todo.IndexView")
	if err := c.Patch(IndexViewItems(key, list)); err != nil {
		return err
	}
	return c.SetSignals(IndexViewSignals{Title: ""})
})

// Clear removes every item and patches the empty list.
var Clear = gx.Action(func(c *gx.Ctx, in route.Clear) error {
	mu.Lock()
	items = nil
	mu.Unlock()
	key := gx.ScopeKey(gx.Scope(c.R), "todo.IndexView")
	return c.Patch(IndexViewItems(key, nil))
})

// Count patches the count. The element has the id that the patch names.
var Count = gx.Action(func(c *gx.Ctx, in route.Count) error {
	mu.Lock()
	n := len(items)
	mu.Unlock()
	return c.Patch(gx.El("span", gx.Attrs{{Key: "id", Value: "todo-count"}}, gx.Text(strconv.Itoa(n)+" items")))
})

// Routes lists every page, action and form of the slice.
var Routes = gx.Collect(IndexPage, Add, Clear, Count)
```

<Result page="guides/actions-and-signals" get="/todo" />

<!-- expect GET /todo
data-signals
Counting
-->

## Automatic updates

`c.Update(node)` takes the component of the action, made from new data. You write no list of fragments.

1. The page gives each fragment a hash of its content, in the attribute `data-gx-h`.
2. The browser sends the hashes with each action request.
3. The server renders the component and compares the hash of each fragment.
4. Only a fragment with a different hash goes to the browser.

The server keeps no state for a user between two requests.

A fragment is the unit of an update. A change inside an inner fragment sends only the inner fragment. A new or a removed inner fragment sends the fragment around it. A component with no fragment goes as a whole, and `gx check` gives the hint [GX4012](/errors/GX4012/) for a large one.

A fragment that holds a link to the current page has a different hash in an action answer, so it always goes. Use `c.Patch` when you know the exact fragment.

## Optimistic updates

An `optimistic:<event>` directive changes signals before the request of the action, so the page shows the result immediately. Write it beside the `on:<event>` handler of the action: `<button on:click={route.Like{}} optimistic:click={$Count++}>`.

1. The runtime saves the value of each signal that the statements write.
2. The statements run. The page shows the new values.
3. The request of the action goes to the server.
4. When the action succeeds, the answer of the server wins. A `c.SetSignals` or a patch replaces the optimistic value. An action with no answer keeps it.
5. When the action fails, the runtime puts the saved values back. A failure is an error of the handler, a rule failure, an error status or no network.

The directive changes signals only. To show an optimistic row or label, bind it to a signal with `show` or `text`.

The request of an optimistic update goes one time. The adapter does not send it again after a failure.

An action with `.debounce` or `.throttle` sends its request later, so its optimistic directive has no rollback. `gx check` reports [GX4013](/errors/GX4013/) for a directive with no action.

## Events and modifiers

An `on:` directive takes a browser event name.

| Modifier | Meaning |
| --- | --- |
| `.prevent` | Calls `preventDefault`. |
| `.stop` | Stops the event at this element. |
| `.once` | Runs one time. |
| `.outside` | Runs for an event outside the element. |
| `.window` | Listens on the window. |
| `.debounce(300ms)` | Waits until the events stop. |
| `.throttle(1s)` | Runs at most one time in the period. |

Three special events invoke a `GET` action with no click: `load`, `visible` and `interval(5s)`. Use them for a part that loads late and for polling.

## One scope for each instance

Each instance of a component has its own signals. Two carts on one page keep two `Qty` values.

A component with signals that renders more than one time needs `key={expr}`. In a loop with no key, that is the diagnostic [GX2012](/errors/GX2012/).

## What a client expression cannot do

- It cannot call a Go function, except the functions of package `gxc`. Read [GX4005](/errors/GX4005/).
- It cannot use a type or an operator that gives a different result in JavaScript. Read [GX4007](/errors/GX4007/).
- It cannot read a `gx.Secret` value. Read [GX7002](/errors/GX7002/).

A value from the server in a client expression goes into the page as JSON. The reader can see it.

## Errors

An action that returns an error shows a toast in production. In `gx dev`, the error overlay shows the error.
