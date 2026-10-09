package starlight

import "github.com/alternayte/gx"

var SidebarItemFixtures = gx.Fixtures[SidebarItemProps]{
	"Link":   {Item: NavItem{Label: "Introduction", Href: gx.URL("/start/"), Badge: "New"}, Path: "/start/"},
	"Group":  {Item: NavItem{Label: "Errors", Items: []NavItem{{Label: "GX1000", Href: gx.URL("/errors/GX1000/")}}}},
	"Closed": {Item: NavItem{Label: "Errors", Collapsed: true, Items: []NavItem{{Label: "GX1000", Href: gx.URL("/errors/GX1000/")}}}},
}
