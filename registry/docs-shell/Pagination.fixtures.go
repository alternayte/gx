package shell

import "github.com/alternayte/gx"

var PaginationFixtures = gx.Fixtures[PaginationProps]{
	"Both":  {Prev: &NavItem{Label: "Start", Href: gx.URL("/start/")}, Next: fixturePage.Next},
	"Next":  {Next: fixturePage.Next},
	"Empty": {},
}
