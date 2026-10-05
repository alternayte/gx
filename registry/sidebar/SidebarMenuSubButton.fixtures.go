package sidebar

import "github.com/alternayte/gx"

var SidebarMenuSubButtonFixtures = gx.Fixtures[SidebarMenuSubButtonProps]{
	"Link":   {Href: gx.URL("/docs/forms"), Children: gx.El("span", nil, gx.Text("Forms"))},
	"Active": {Href: gx.URL("/docs/start"), Active: true, Children: gx.El("span", nil, gx.Text("Get started"))},
	"Small":  {Href: gx.URL("/docs/forms"), Size: Sm, Children: gx.El("span", nil, gx.Text("Forms"))},
}

// SidebarMenuSubButtonWrap renders the part in a sub menu, where it belongs.
func SidebarMenuSubButtonWrap(n gx.Node) gx.Node {
	return gx.El("div", gx.Attrs{{Key: "class", Value: "w-64"}}, SidebarMenuSub(SidebarMenuSubProps{Children: SidebarMenuSubItem(SidebarMenuSubItemProps{Children: n})}))
}
