package pagination

import "github.com/alternayte/gx"

var paginationOne = gx.URL("/?page=1")
var paginationTwo = gx.URL("/?page=2")

var PaginationFixtures = gx.Fixtures[PaginationProps]{
	"Trail": {Children: PaginationContent(PaginationContentProps{Children: gx.Frag(
		PaginationItem(PaginationItemProps{Children: PaginationPrevious(PaginationPreviousProps{Href: paginationOne})}),
		PaginationItem(PaginationItemProps{Children: PaginationLink(PaginationLinkProps{Href: paginationOne, Active: true, Children: gx.Text("1")})}),
		PaginationItem(PaginationItemProps{Children: PaginationLink(PaginationLinkProps{Href: paginationTwo, Children: gx.Text("2")})}),
		PaginationItem(PaginationItemProps{Children: PaginationEllipsis(PaginationEllipsisProps{})}),
		PaginationItem(PaginationItemProps{Children: PaginationNext(PaginationNextProps{Href: paginationTwo})}),
	)})},
}

var PaginationContentFixtures = gx.Fixtures[PaginationContentProps]{"Empty": {}}
var PaginationItemFixtures = gx.Fixtures[PaginationItemProps]{"Empty": {}}
var PaginationLinkFixtures = gx.Fixtures[PaginationLinkProps]{
	"Inactive": {Href: paginationTwo, Children: gx.Text("2")},
	"Active":   {Href: paginationOne, Active: true, Children: gx.Text("1")},
}
var PaginationPreviousFixtures = gx.Fixtures[PaginationPreviousProps]{"Default": {Href: paginationOne}}
var PaginationNextFixtures = gx.Fixtures[PaginationNextProps]{"Default": {Href: paginationTwo}}
var PaginationEllipsisFixtures = gx.Fixtures[PaginationEllipsisProps]{"Default": {}}
