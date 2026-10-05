package sidebar

import "github.com/alternayte/gx"

var SidebarMenuBadgeFixtures = gx.Fixtures[SidebarMenuBadgeProps]{"Count": {Children: gx.Text("12")}}

// SidebarMenuBadgeWrap renders the part in a menu, where it belongs.
func SidebarMenuBadgeWrap(n gx.Node) gx.Node {
	return SidebarMenu(SidebarMenuProps{Class: "w-64", Children: SidebarMenuItem(SidebarMenuItemProps{Children: gx.Frag(SidebarMenuButton(SidebarMenuButtonProps{Href: gx.URL("/projects"), Children: gx.El("span", nil, gx.Text("Projects"))}), n)})})
}
