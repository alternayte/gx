// Package route holds the dashboard routes (DR-01).
package route

import "github.com/alternayte/gx"

// Page is the dashboard page. It shows the TypeScript islands of the shop.
type Page struct {
	gx.Route `GET /dashboard`
}

// Refresh patches the chart panel with the numbers of the next round.
type Refresh struct {
	gx.Route `POST /dashboard/refresh`
	Round    int `query:"round"`
}
