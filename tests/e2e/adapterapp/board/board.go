// Package board is the one slice of the adapter contract app: one action
// for each answer kind, each event modifier, a form and two pages in one
// layout. It has no signals.
package board

import (
	"errors"
	"strconv"
	"sync"

	"adapterapp/board/route"

	"github.com/alternayte/gx"
)

// state is the server state of the app. The suite runs one browser page at
// a time.
var state = struct {
	sync.Mutex
	inc, last, first, tick int
	counts                 map[string]int
}{counts: map[string]int{}}

// A patch names its target by the id of its root element, so each patched
// node is one element with a fixed id.

// value is a number with an id.
func value(id string, n int) gx.Node {
	return gx.El("output", gx.Attrs{{Key: "id", Value: id}}, gx.Text(strconv.Itoa(n)))
}

// item is the node of the append and the prepend patches. The adapter puts
// it inside the element with the same id.
func item(label string) gx.Node {
	return gx.El("div", gx.Attrs{{Key: "id", Value: "items"}}, gx.Text(label))
}

func swapped(label string) gx.Node {
	return gx.El("p", gx.Attrs{{Key: "id", Value: "swap"}}, gx.Text(label))
}

func gone() gx.Node {
	return gx.El("p", gx.Attrs{{Key: "id", Value: "gone"}}, gx.Text("here"))
}

// Layout wraps each page in the shell.
var Layout = gx.Layout(nil, func(_ struct{}, children gx.Node) gx.Node {
	return Shell(ShellProps{Children: children})
})

var homePage = gx.Page(func(c *gx.Ctx, in route.Home) (HomeProps, error) {
	state.Lock()
	defer state.Unlock()
	return HomeProps{Inc: state.inc}, nil
}, Home)

var aboutPage = gx.Page(func(c *gx.Ctx, in route.About) (AboutProps, error) {
	return AboutProps{}, nil
}, About)

var inc = gx.Action(func(c *gx.Ctx, in route.Inc) error {
	state.Lock()
	state.inc++
	n := state.inc
	state.Unlock()
	return c.Patch(value("inc-value", n))
})

var addLast = gx.Action(func(c *gx.Ctx, in route.AddLast) error {
	state.Lock()
	state.last++
	n := state.last
	state.Unlock()
	return c.Patch(gx.Append, item("last "+strconv.Itoa(n)))
})

var addFirst = gx.Action(func(c *gx.Ctx, in route.AddFirst) error {
	state.Lock()
	state.first++
	n := state.first
	state.Unlock()
	return c.Patch(gx.Prepend, item("first "+strconv.Itoa(n)))
})

var swap = gx.Action(func(c *gx.Ctx, in route.Swap) error {
	return c.Patch(gx.Replace, swapped("new"))
})

var drop = gx.Action(func(c *gx.Ctx, in route.Drop) error {
	return c.Patch(gx.Remove, gone())
})

var leave = gx.Action(func(c *gx.Ctx, in route.Leave) error {
	return c.Redirect(route.About{})
})

var notify = gx.Action(func(c *gx.Ctx, in route.Notify) error {
	return c.Toast("Saved", gx.ToastSuccess)
})

var fail = gx.Action(func(c *gx.Ctx, in route.Fail) error {
	return errors.New("the board is full")
})

var quiet = gx.Action(func(c *gx.Ctx, in route.Quiet) error { return nil })

var fade = gx.Action(func(c *gx.Ctx, in route.Fade) error {
	return c.Patch(gx.ViewTransition, value("fade-value", 1))
})

var lazy = gx.Action(func(c *gx.Ctx, in route.Lazy) error {
	return c.Patch(value("lazy-value", 1))
})

var seen = gx.Action(func(c *gx.Ctx, in route.Seen) error {
	return c.Patch(value("seen-value", 1))
})

var tick = gx.Action(func(c *gx.Ctx, in route.Tick) error {
	state.Lock()
	state.tick++
	n := state.tick
	state.Unlock()
	return c.Patch(value("tick-value", n))
})

var count = gx.Action(func(c *gx.Ctx, in route.Count) error {
	state.Lock()
	state.counts[in.Name]++
	n := state.counts[in.Name]
	state.Unlock()
	return c.Patch(value(in.Name+"-value", n))
})

var joinPage = gx.Page(func(c *gx.Ctx, in route.JoinPage) (JoinViewProps, error) {
	return join.Props(&route.Join{}), nil
}, JoinView)

var join = gx.Form(func(c *gx.Ctx, in *route.Join) error {
	if in.Name == "taken" {
		return gx.FieldError(&in.Name, "name.taken")
	}
	return c.Redirect(route.About{})
}, JoinView)

// Routes lists every page, action and form of the slice.
var Routes = gx.Collect(homePage, aboutPage, inc, addLast, addFirst, swap, drop, leave, notify, fail, quiet,
	fade, lazy, seen, tick, count, joinPage, join)
