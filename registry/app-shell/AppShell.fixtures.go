package appshell

import (
	"github.com/alternayte/gx"
	"github.com/alternayte/gx/registry/sidebar"
)

var AppShellFixtures = gx.Fixtures[AppShellProps]{
	"Default": {
		Title: "Acme",
		Nav: sidebar.SidebarGroup(sidebar.SidebarGroupProps{Title: "Menu", Children: gx.Frag(
			sidebar.SidebarItem(sidebar.SidebarItemProps{Href: gx.URL("/"), Active: true, Children: gx.Text("Home")}),
			sidebar.SidebarItem(sidebar.SidebarItemProps{Href: gx.URL("/docs"), Children: gx.Text("Docs")}),
		)}),
		Children: gx.Text("Page content."),
	},
}
