package navigationmenu

import "github.com/alternayte/gx"

var NavigationMenuContentFixtures = gx.Fixtures[NavigationMenuContentProps]{
	"Default": {Children: gx.El("ul", gx.Attrs{{Key: "class", Value: "grid w-48 gap-1"}},
		gx.El("li", nil, NavigationMenuLink(NavigationMenuLinkProps{Href: gx.URL("/products"), Children: gx.Text("All products")})),
		gx.El("li", nil, NavigationMenuLink(NavigationMenuLinkProps{Href: gx.URL("/products/new"), Children: gx.Text("New arrivals")})),
	)},
}

// NavigationMenuContentWrap renders the content under a trigger, as a page
// uses it. Hover or focus the trigger to show the content.
func NavigationMenuContentWrap(n gx.Node) gx.Node {
	return NavigationMenu(NavigationMenuProps{Label: "Content", Children: NavigationMenuItem(NavigationMenuItemProps{Children: gx.Frag(
		NavigationMenuTrigger(NavigationMenuTriggerProps{Children: gx.Text("Products")}),
		n,
	)})})
}
