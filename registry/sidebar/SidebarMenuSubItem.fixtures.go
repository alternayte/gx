package sidebar

import "github.com/alternayte/gx"

var SidebarMenuSubItemFixtures = gx.Fixtures[SidebarMenuSubItemProps]{"Empty": {}}

// SidebarMenuSubItemWrap renders the part in a sub menu, where it belongs.
func SidebarMenuSubItemWrap(n gx.Node) gx.Node {
	return gx.El("div", gx.Attrs{{Key: "class", Value: "w-64"}}, SidebarMenuSub(SidebarMenuSubProps{Children: n}))
}
