package sidebar

import "github.com/alternayte/gx"

var SidebarGroupActionFixtures = gx.Fixtures[SidebarGroupActionProps]{"Add": {Label: "Project options", Children: more()}}

// SidebarGroupActionWrap renders the action in a group, which positions
// it.
func SidebarGroupActionWrap(n gx.Node) gx.Node {
	return SidebarGroup(SidebarGroupProps{Class: "w-64", Children: gx.Frag(
		SidebarGroupLabel(SidebarGroupLabelProps{Children: gx.Text("Projects")}),
		n,
	)})
}
