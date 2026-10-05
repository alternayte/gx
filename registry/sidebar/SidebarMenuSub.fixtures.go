package sidebar

import "github.com/alternayte/gx"

var SidebarMenuSubFixtures = gx.Fixtures[SidebarMenuSubProps]{
	"Two": {Children: gx.Frag(
		SidebarMenuSubItem(SidebarMenuSubItemProps{Children: SidebarMenuSubButton(SidebarMenuSubButtonProps{Href: gx.URL("/docs/start"), Active: true, Children: gx.El("span", nil, gx.Text("Get started"))})}),
		SidebarMenuSubItem(SidebarMenuSubItemProps{Children: SidebarMenuSubButton(SidebarMenuSubButtonProps{Href: gx.URL("/docs/forms"), Size: Sm, Children: gx.El("span", nil, gx.Text("Forms"))})}),
	)},
}

// SidebarMenuSubWrap renders the part in a menu, where it belongs.
func SidebarMenuSubWrap(n gx.Node) gx.Node {
	return SidebarMenu(SidebarMenuProps{Class: "w-64", Children: SidebarMenuItem(SidebarMenuItemProps{Children: n})})
}
