package sidebar

import "github.com/alternayte/gx"

var SidebarMenuFixtures = gx.Fixtures[SidebarMenuProps]{
	"Nested": {Class: "w-64", Children: gx.Frag(
		SidebarMenuItem(SidebarMenuItemProps{Children: gx.Frag(
			SidebarMenuButton(SidebarMenuButtonProps{Href: gx.URL("/docs"), Children: gx.El("span", nil, gx.Text("Docs"))}),
			SidebarMenuBadge(SidebarMenuBadgeProps{Children: gx.Text("12")}),
			SidebarMenuSub(SidebarMenuSubProps{Children: gx.Frag(
				SidebarMenuSubItem(SidebarMenuSubItemProps{Children: SidebarMenuSubButton(SidebarMenuSubButtonProps{Href: gx.URL("/docs/start"), Active: true, Children: gx.El("span", nil, gx.Text("Get started"))})}),
				SidebarMenuSubItem(SidebarMenuSubItemProps{Children: SidebarMenuSubButton(SidebarMenuSubButtonProps{Href: gx.URL("/docs/forms"), Children: gx.El("span", nil, gx.Text("Forms"))})}),
			)}),
		)}),
		SidebarMenuItem(SidebarMenuItemProps{Children: gx.Frag(
			SidebarMenuButton(SidebarMenuButtonProps{Href: gx.URL("/projects"), Children: gx.El("span", nil, gx.Text("Projects"))}),
			SidebarMenuAction(SidebarMenuActionProps{Label: "Project options", Children: more()}),
		)}),
	)},
	"Loading": {Class: "w-64", Children: gx.Frag(
		SidebarMenuItem(SidebarMenuItemProps{Children: SidebarMenuSkeleton(SidebarMenuSkeletonProps{ShowIcon: true, Width: "80%"})}),
		SidebarMenuItem(SidebarMenuItemProps{Children: SidebarMenuSkeleton(SidebarMenuSkeletonProps{ShowIcon: true, Width: "55%"})}),
		SidebarMenuItem(SidebarMenuItemProps{Children: SidebarMenuSkeleton(SidebarMenuSkeletonProps{ShowIcon: true, Width: "70%"})}),
	)},
}
