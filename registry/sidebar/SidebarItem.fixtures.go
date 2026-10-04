package sidebar

import "github.com/alternayte/gx"

var SidebarItemFixtures = gx.Fixtures[SidebarItemProps]{"Item": {Href: gx.URL("/"), Children: gx.Text("Home")}}
