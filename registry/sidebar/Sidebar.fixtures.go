package sidebar

import "github.com/alternayte/gx"

var SidebarFixtures = gx.Fixtures[SidebarProps]{
	"Full": {Id: "demo-sidebar", Class: "h-72", Children: gx.Frag(
		SidebarHeader(SidebarHeaderProps{Children: gx.Text("Gx")}),
		SidebarContent(SidebarContentProps{Children: SidebarGroup(SidebarGroupProps{Title: "Menu", Children: gx.Frag(
			SidebarItem(SidebarItemProps{Href: gx.URL("/"), Active: true, Children: gx.Text("Home")}),
			SidebarItem(SidebarItemProps{Href: gx.URL("/docs"), Children: gx.Text("Docs")}),
		)})}),
		SidebarFooter(SidebarFooterProps{Children: gx.Text("v0.1.0")}),
	)},
	"Hidden": {Id: "demo-sidebar-hidden", Attrs: gx.Attrs{gx.Bool("hidden", true)}},
}
