package dropdownmenu

import "github.com/alternayte/gx"

var DropdownMenuSubFixtures = gx.Fixtures[DropdownMenuSubProps]{
	"Default": {Children: gx.Frag(
		DropdownMenuSubTrigger(DropdownMenuSubTriggerProps{Children: gx.Text("More tools")}),
		DropdownMenuSubContent(DropdownMenuSubContentProps{Children: gx.Frag(
			DropdownMenuItem(DropdownMenuItemProps{Children: gx.Text("Save page")}),
			DropdownMenuItem(DropdownMenuItemProps{Children: gx.Text("Create shortcut")}),
		)}),
	)},
}

// DropdownMenuSubWrap renders the sub-menu inside a menu, as a page uses it.
func DropdownMenuSubWrap(n gx.Node) gx.Node {
	return gx.El("div", gx.Attrs{{Key: "role", Value: "menu"}, {Key: "class", Value: "w-56"}}, n)
}
