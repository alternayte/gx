package navigationmenu

import "github.com/alternayte/gx"

var NavigationMenuItemFixtures = gx.Fixtures[NavigationMenuItemProps]{
	"Item":   {Href: gx.URL("/docs"), Children: gx.Text("Docs")},
	"Active": {Href: gx.URL("/"), Active: true, Children: gx.Text("Home")},
}
