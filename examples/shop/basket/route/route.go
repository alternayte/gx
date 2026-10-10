// Package route holds the basket routes (DR-01).
package route

import "github.com/alternayte/gx"

// Page is the basket page. Its actions answer with c.Update.
type Page struct {
	gx.Route `GET /basket`
}

// Bump adds one to the quantity of a line.
type Bump struct {
	gx.Route `POST /basket/bump/{sku}`
	SKU      string
}

// Reset puts the basket back to its first lines.
type Reset struct {
	gx.Route `POST /basket/reset`
}

// Star adds a star. The page shows it before the answer (REQ-ACT-18).
type Star struct {
	gx.Route `POST /basket/star`
}

// StarFail is an action that always fails.
type StarFail struct {
	gx.Route `POST /basket/star-fail`
}

// StarSet sets the stars to the number of the server.
type StarSet struct {
	gx.Route `POST /basket/star-set`
}

// SetQty sets the quantity of a line. It has rules, so a call with a bad
// quantity gets a field error (REQ-ACT-19).
type SetQty struct {
	gx.Route `POST /basket/qty/{sku}`
	SKU      string
	Qty      int
}

// Rules declares the limits of the quantity.
func (in *SetQty) Rules() gx.Rules {
	return gx.Rules{gx.Field(&in.Qty, gx.Min(1), gx.Max(99))}
}

// Count reads the number of items of the basket.
type Count struct {
	gx.Route `GET /basket/count`
}
