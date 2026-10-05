package sidebar

import "github.com/alternayte/gx"

var SidebarMenuSkeletonFixtures = gx.Fixtures[SidebarMenuSkeletonProps]{
	"Text": {},
	"Icon": {ShowIcon: true, Width: "55%"},
}

// SidebarMenuSkeletonWrap renders the part in a menu, where it belongs.
func SidebarMenuSkeletonWrap(n gx.Node) gx.Node {
	return SidebarMenu(SidebarMenuProps{Class: "w-64", Children: SidebarMenuItem(SidebarMenuItemProps{Children: n})})
}
