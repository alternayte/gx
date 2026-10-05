package dropdownmenu

import "github.com/alternayte/gx"

var DropdownMenuSubContentFixtures = gx.Fixtures[DropdownMenuSubContentProps]{
	"Content": {Class: "w-48", Children: gx.Frag(
		DropdownMenuItem(DropdownMenuItemProps{Children: gx.Text("Save page")}),
		DropdownMenuItem(DropdownMenuItemProps{Children: gx.Text("Create shortcut")}),
		DropdownMenuSeparator(DropdownMenuSeparatorProps{}),
		DropdownMenuItem(DropdownMenuItemProps{Children: gx.Text("Developer tools")}),
	)},
}

// DropdownMenuSubContentWrap renders the content behind its trigger inside a menu,
// as a page uses it.
func DropdownMenuSubContentWrap(n gx.Node) gx.Node {
	return gx.El("div", gx.Attrs{{Key: "role", Value: "menu"}, {Key: "class", Value: "w-56"}},
		DropdownMenuSub(DropdownMenuSubProps{Children: gx.Frag(
			DropdownMenuSubTrigger(DropdownMenuSubTriggerProps{Children: gx.Text("More tools")}),
			n,
		)}),
	)
}
