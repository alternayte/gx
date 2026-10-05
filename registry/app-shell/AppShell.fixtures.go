package appshell

import (
	"github.com/alternayte/gx"
	"github.com/alternayte/gx/registry/sidebar"
)

var AppShellFixtures = gx.Fixtures[AppShellProps]{
	"Default": {
		Title: "Acme",
		Nav: sidebar.SidebarGroup(sidebar.SidebarGroupProps{Children: gx.Frag(
			sidebar.SidebarGroupLabel(sidebar.SidebarGroupLabelProps{Children: gx.Text("Menu")}),
			sidebar.SidebarGroupContent(sidebar.SidebarGroupContentProps{Children: sidebar.SidebarMenu(sidebar.SidebarMenuProps{Children: gx.Frag(
				sidebar.SidebarMenuItem(sidebar.SidebarMenuItemProps{Children: sidebar.SidebarMenuButton(sidebar.SidebarMenuButtonProps{Href: gx.URL("/"), Active: true, Children: gx.El("span", nil, gx.Text("Home"))})}),
				sidebar.SidebarMenuItem(sidebar.SidebarMenuItemProps{Children: sidebar.SidebarMenuButton(sidebar.SidebarMenuButtonProps{Href: gx.URL("/docs"), Children: gx.El("span", nil, gx.Text("Docs"))})}),
			)})}),
		)}),
		Footer:   gx.El("span", gx.Attrs{{Key: "class", Value: "px-2 text-xs text-sidebar-foreground/70"}}, gx.Text("v0.1.0")),
		Children: gx.Text("Page content."),
	},
}
