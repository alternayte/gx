package sidebar

import "github.com/alternayte/gx"

var SidebarMenuActionFixtures = gx.Fixtures[SidebarMenuActionProps]{
	"Add":   {Label: "Project options", Children: more()},
	"Hover": {Label: "Project options", ShowOnHover: true, Children: more()},
}

// SidebarMenuActionWrap renders the part in a menu, where it belongs.
func SidebarMenuActionWrap(n gx.Node) gx.Node {
	return SidebarMenu(SidebarMenuProps{Class: "w-64", Children: SidebarMenuItem(SidebarMenuItemProps{Children: gx.Frag(SidebarMenuButton(SidebarMenuButtonProps{Href: gx.URL("/projects"), Children: gx.El("span", nil, gx.Text("Projects"))}), n)})})
}
