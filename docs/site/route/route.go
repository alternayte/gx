// Package route holds the page routes of the docs site slice (DR-01).
package route

import "github.com/alternayte/gx"

// Preview is the page of one live example. A component page shows it in a
// frame, so an overlay or a fixed element stays inside the frame
// (REQ-DOC-02).
type Preview struct {
	gx.Route `GET /preview/{item}/{example}/`
	Item     string
	Example  string
}

// TableDemo is one state of the live data table example: a status filter,
// a sort and a page. The static export writes one page for each state, so
// a sort link, a paging link and a filter link work with no server
// (REQ-DOC-02). Item is "data-table" or "data-table-page". Sort is "none"
// for the order of the data.
type TableDemo struct {
	gx.Route `GET /preview/{item}/live/{status}/{sort}/{dir}/{page}/`
	Item     string
	Status   string
	Sort     string
	Dir      string
	Page     int
}

// Result is the captured page of one request to the sample app of a guide.
// The guide shows it in a frame (REQ-DOC-04). Key names the capture.
type Result struct {
	gx.Route `GET /result/{key}/`
	Key      string
}
