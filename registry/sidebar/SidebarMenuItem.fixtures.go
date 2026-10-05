package sidebar

import "github.com/alternayte/gx"

var SidebarMenuItemFixtures = gx.Fixtures[SidebarMenuItemProps]{"Empty": {}}

// SidebarMenuItemWrap renders the part in a menu, where it belongs.
func SidebarMenuItemWrap(n gx.Node) gx.Node {
	return SidebarMenu(SidebarMenuProps{Class: "w-64", Children: n})
}
