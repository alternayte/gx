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
