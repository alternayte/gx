package navigationmenu

import "github.com/alternayte/gx"

var NavigationMenuFixtures = gx.Fixtures[NavigationMenuProps]{
	"Default": {Children: gx.Frag(
		NavigationMenuItem(NavigationMenuItemProps{Href: gx.URL("/"), Active: true, Children: gx.Text("Home")}),
		NavigationMenuItem(NavigationMenuItemProps{Href: gx.URL("/docs"), Children: gx.Text("Docs")}),
	)},
}
