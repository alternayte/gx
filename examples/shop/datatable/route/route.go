// Package route holds the data table route (DR-01).
package route

import "github.com/alternayte/gx"

// List is the server-driven data table page.
type List struct {
	gx.Route `GET /datatable`
	Q        string `query:"q"`
	Sort     string `query:"sort" default:"name"`
	Dir      string `query:"dir" default:"asc"`
	Page     int    `query:"page" default:"1"`
}
