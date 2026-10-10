// Package basket shows automatic updates: an action gives c.Update the
// component from the new data, and only the fragments that changed go to
// the browser (REQ-ACT-15, REQ-ACT-16).
package basket

import (
	"errors"
	"sync"
	"time"

	"github.com/alternayte/gx"
	"github.com/alternayte/gx/examples/shop/basket/route"
)

// Line is one line of the basket.
type Line struct {
	SKU  string
	Name string
	Qty  int
}

// store holds the lines of the one basket of the example. A real app reads
// them from its database.
var store = struct {
	sync.Mutex
	lines []Line
}{lines: firstLines()}

func firstLines() []Line {
	return []Line{{SKU: "tea", Name: "Tea", Qty: 1}, {SKU: "milk", Name: "Milk", Qty: 2}, {SKU: "rice", Name: "Rice", Qty: 1}}
}

// lines returns a copy of the lines of the basket.
func lines() []Line {
	store.Lock()
	defer store.Unlock()
	return append([]Line(nil), store.lines...)
}

// count returns the number of items of the lines.
func count(ls []Line) int {
	n := 0
	for _, l := range ls {
		n += l.Qty
	}
	return n
}

// BasketPage is the basket page.
var BasketPage = gx.Page(
	func(c *gx.Ctx, in route.Page) (BasketViewProps, error) { return BasketViewProps{Lines: lines()}, nil },
	BasketView)

// Bump adds one to a line and gives the whole basket to c.Update. The line
// and the count are the fragments that differ, so only they go out.
var Bump = gx.Action(func(c *gx.Ctx, in route.Bump) error {
	store.Lock()
	for i := range store.lines {
		if store.lines[i].SKU == in.SKU {
			store.lines[i].Qty++
		}
	}
	store.Unlock()
	return c.Update(Basket(BasketProps{Lines: lines()}))
})

// Totals is the JSON result of the basket actions for a client that is not
// a page (REQ-ACT-19).
type Totals struct {
	// Items is the number of items of the basket.
	Items int `json:"items"`
	// Lines holds the quantity of each line, by SKU.
	Lines []LineQty `json:"lines"`
}

// LineQty is the quantity of one line.
type LineQty struct {
	SKU string `json:"sku"`
	Qty int    `json:"qty"`
}

func totals() Totals {
	ls := lines()
	out := Totals{Items: count(ls), Lines: make([]LineQty, len(ls))}
	for i, l := range ls {
		out.Lines[i] = LineQty{SKU: l.SKU, Qty: l.Qty}
	}
	return out
}

// Sets the quantity of one line of the basket. A page gets the fragments
// that changed, and a JSON client gets the totals.
var SetQty = gx.Action(func(c *gx.Ctx, in route.SetQty) error {
	found := false
	store.Lock()
	for i := range store.lines {
		if store.lines[i].SKU == in.SKU {
			store.lines[i].Qty, found = in.Qty, true
		}
	}
	store.Unlock()
	if !found {
		return gx.NotFound()
	}
	gx.ToolResult(c, totals())
	return c.Update(Basket(BasketProps{Lines: lines()}))
}).API()

// Reads the totals of the basket.
var Count = gx.Action(func(c *gx.Ctx, in route.Count) error {
	gx.ToolResult(c, totals())
	return nil
}).API()

// Reset puts the first lines back.
var Reset = gx.Action(func(c *gx.Ctx, in route.Reset) error {
	store.Lock()
	store.lines = firstLines()
	store.Unlock()
	return c.Update(Basket(BasketProps{Lines: lines()}))
})

// Star answers with nothing: the optimistic value of the page stays.
var Star = gx.Action(func(c *gx.Ctx, in route.Star) error { return nil })

// StarFail fails after a moment, so the page shows the optimistic value and
// then the value of before.
var StarFail = gx.Action(func(c *gx.Ctx, in route.StarFail) error {
	time.Sleep(150 * time.Millisecond)
	return errors.New("the shop did not save the star")
})

// StarSet answers with its own number. The answer of the server wins over
// the optimistic value.
var StarSet = gx.Action(func(c *gx.Ctx, in route.StarSet) error {
	return c.SetSignals(StarsSignals{Stars: 100})
})

// Routes collects the basket page and its actions.
var Routes = gx.Collect(BasketPage, Bump, Reset, Star, StarFail, StarSet, SetQty, Count)
