package pagination

import "github.com/alternayte/gx"

var PaginationLinkFixtures = gx.Fixtures[PaginationLinkProps]{
	"Inactive": {Href: gx.URL("/?page=2"), Children: gx.Text("2")},
	"Active":   {Href: gx.URL("/?page=1"), Active: true, Children: gx.Text("1")},
}
