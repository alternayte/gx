package navigationmenu

import "github.com/alternayte/gx"

var NavigationMenuItemFixtures = gx.Fixtures[NavigationMenuItemProps]{
	"Link": {Children: NavigationMenuLink(NavigationMenuLinkProps{Href: gx.URL("/docs"), Variant: Trigger, Children: gx.Text("Docs")})},
}

// NavigationMenuItemWrap renders the item inside a menu, as a page uses it.
func NavigationMenuItemWrap(n gx.Node) gx.Node {
	return NavigationMenu(NavigationMenuProps{Label: "Item", Children: n})
}
