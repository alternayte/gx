package sidebar

import "github.com/alternayte/gx"

var SidebarGroupFixtures = gx.Fixtures[SidebarGroupProps]{
	"Labelled": {Class: "w-64 bg-sidebar text-sidebar-foreground", Children: gx.Frag(
		SidebarGroupLabel(SidebarGroupLabelProps{Children: gx.Text("Projects")}),
		SidebarGroupAction(SidebarGroupActionProps{Label: "Project options", Children: more()}),
		SidebarGroupContent(SidebarGroupContentProps{Children: demoMenu()}),
	)},
}
