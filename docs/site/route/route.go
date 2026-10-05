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
