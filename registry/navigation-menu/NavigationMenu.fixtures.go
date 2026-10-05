package navigationmenu

import "github.com/alternayte/gx"

var NavigationMenuFixtures = gx.Fixtures[NavigationMenuProps]{
	"Default": {Label: "Main", Children: gx.Frag(
		NavigationMenuItem(NavigationMenuItemProps{Children: gx.Frag(
			NavigationMenuTrigger(NavigationMenuTriggerProps{Children: gx.Text("Products")}),
			NavigationMenuContent(NavigationMenuContentProps{Children: gx.El("ul", gx.Attrs{{Key: "class", Value: "grid w-48 gap-1"}},
				gx.El("li", nil, NavigationMenuLink(NavigationMenuLinkProps{Href: gx.URL("/products"), Children: gx.Text("All products")})),
				gx.El("li", nil, NavigationMenuLink(NavigationMenuLinkProps{Href: gx.URL("/products/new"), Children: gx.Text("New arrivals")})),
			)}),
		)}),
		NavigationMenuItem(NavigationMenuItemProps{Children: NavigationMenuLink(NavigationMenuLinkProps{Href: gx.URL("/"), Variant: Trigger, Active: true, Children: gx.Text("Home")})}),
		NavigationMenuItem(NavigationMenuItemProps{Children: NavigationMenuLink(NavigationMenuLinkProps{Href: gx.URL("/docs"), Variant: Trigger, Children: gx.Text("Docs")})}),
	)},
}
