package pagination

import "github.com/alternayte/gx"

var PaginationFixtures = gx.Fixtures[PaginationProps]{
	"Trail": {Children: PaginationContent(PaginationContentProps{Children: gx.Frag(
		PaginationItem(PaginationItemProps{Children: PaginationPrevious(PaginationPreviousProps{Href: gx.URL("/?page=1")})}),
		PaginationItem(PaginationItemProps{Children: PaginationLink(PaginationLinkProps{Href: gx.URL("/?page=1"), Active: true, Children: gx.Text("1")})}),
		PaginationItem(PaginationItemProps{Children: PaginationLink(PaginationLinkProps{Href: gx.URL("/?page=2"), Children: gx.Text("2")})}),
		PaginationItem(PaginationItemProps{Children: PaginationEllipsis(PaginationEllipsisProps{})}),
		PaginationItem(PaginationItemProps{Children: PaginationNext(PaginationNextProps{Href: gx.URL("/?page=2")})}),
	)})},
}
