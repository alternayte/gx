package shell

import "github.com/alternayte/gx"

var SidebarItemFixtures = gx.Fixtures[SidebarItemProps]{
	"Link":   {Item: NavItem{Label: "Routing", Href: gx.URL("/guides/routing/")}, Path: "/guides/routing/"},
	"Nested": {Item: fixtureRouting, Path: "/guides/routing/pages/"},
}
