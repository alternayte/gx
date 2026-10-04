package pagination

import "github.com/alternayte/gx"

var PaginationItemFixtures = gx.Fixtures[PaginationItemProps]{"Empty": {}}

// PaginationItemWrap renders the item inside a list, as a page uses it.
func PaginationItemWrap(n gx.Node) gx.Node {
	return PaginationContent(PaginationContentProps{Children: n})
}
