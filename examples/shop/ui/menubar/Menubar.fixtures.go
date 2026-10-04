package menubar

import "github.com/alternayte/gx"

var MenubarFixtures = gx.Fixtures[MenubarProps]{
	"Default": {Children: gx.Frag(
		MenubarItem(MenubarItemProps{Href: gx.URL("/"), Active: true, Children: gx.Text("Home")}),
		MenubarItem(MenubarItemProps{Href: gx.URL("/docs"), Children: gx.Text("Docs")}),
	)},
}
