package sidebar

import (
	"github.com/alternayte/gx"
	"github.com/alternayte/gx/registry/icons"
)

// more is the icon of the action fixtures.
func more() gx.Node {
	return icons.Ellipsis(icons.EllipsisProps{})
}

// demoMenu is the menu of the sidebar fixtures.
func demoMenu() gx.Node {
	return SidebarMenu(SidebarMenuProps{Children: gx.Frag(
		SidebarMenuItem(SidebarMenuItemProps{Children: SidebarMenuButton(SidebarMenuButtonProps{Href: gx.URL("/"), Active: true, Children: gx.El("span", nil, gx.Text("Home"))})}),
		SidebarMenuItem(SidebarMenuItemProps{Children: SidebarMenuButton(SidebarMenuButtonProps{Href: gx.URL("/docs"), Children: gx.El("span", nil, gx.Text("Docs"))})}),
	)})
}

var SidebarFixtures = gx.Fixtures[SidebarProps]{
	"Full": {Id: "demo-sidebar", Class: "h-72", Children: gx.Frag(
		SidebarHeader(SidebarHeaderProps{Children: gx.El("span", gx.Attrs{{Key: "class", Value: "px-2 text-sm font-semibold"}}, gx.Text("Gx"))}),
		SidebarContent(SidebarContentProps{Children: SidebarGroup(SidebarGroupProps{Children: gx.Frag(
			SidebarGroupLabel(SidebarGroupLabelProps{Children: gx.Text("Menu")}),
			SidebarGroupContent(SidebarGroupContentProps{Children: demoMenu()}),
		)})}),
		SidebarFooter(SidebarFooterProps{Children: gx.El("span", gx.Attrs{{Key: "class", Value: "px-2 text-xs text-sidebar-foreground/70"}}, gx.Text("v0.1.0"))}),
	)},
	"Right": {Id: "demo-sidebar-right", Side: Right, Class: "h-40", Children: SidebarContent(SidebarContentProps{
		Children: SidebarGroup(SidebarGroupProps{Children: demoMenu()}),
	})},
	"Hidden": {Id: "demo-sidebar-hidden", Attrs: gx.Attrs{gx.Bool("hidden", true)}},
}
