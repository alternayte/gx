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
