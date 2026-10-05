package sidebar

import "github.com/alternayte/gx"

var SidebarMenuButtonFixtures = gx.Fixtures[SidebarMenuButtonProps]{
	"Link":    {Href: gx.URL("/docs"), Children: gx.El("span", nil, gx.Text("Docs"))},
	"Active":  {Href: gx.URL("/"), Active: true, Children: gx.El("span", nil, gx.Text("Home"))},
	"Button":  {Children: gx.El("span", nil, gx.Text("Sign out"))},
	"Outline": {Variant: Outline, Href: gx.URL("/docs"), Children: gx.El("span", nil, gx.Text("Docs"))},
	"Small":   {Size: Sm, Href: gx.URL("/docs"), Children: gx.El("span", nil, gx.Text("Docs"))},
	"Large":   {Size: Lg, Href: gx.URL("/docs"), Children: gx.El("span", nil, gx.Text("Docs"))},
}

// SidebarMenuButtonWrap renders the part in a menu, where it belongs.
func SidebarMenuButtonWrap(n gx.Node) gx.Node {
	return SidebarMenu(SidebarMenuProps{Class: "w-64", Children: SidebarMenuItem(SidebarMenuItemProps{Children: n})})
}
