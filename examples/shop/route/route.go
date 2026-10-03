// Package route holds the page routes of the shop slice (DR-01).
package route

import "github.com/alternayte/gx"

// Home is the shop home page.
type Home struct {
	gx.Route `GET /`
}

// About is the shop about page.
type About struct {
	gx.Route `GET /about`
}
