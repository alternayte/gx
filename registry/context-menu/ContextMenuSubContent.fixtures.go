package contextmenu

import "github.com/alternayte/gx"

var ContextMenuSubContentFixtures = gx.Fixtures[ContextMenuSubContentProps]{
	"Content": {Class: "w-48", Children: gx.Frag(
		ContextMenuItem(ContextMenuItemProps{Children: gx.Text("Save page")}),
		ContextMenuItem(ContextMenuItemProps{Children: gx.Text("Create shortcut")}),
		ContextMenuSeparator(ContextMenuSeparatorProps{}),
		ContextMenuItem(ContextMenuItemProps{Children: gx.Text("Developer tools")}),
	)},
}

// ContextMenuSubContentWrap renders the content behind its trigger inside a menu,
// as a page uses it.
func ContextMenuSubContentWrap(n gx.Node) gx.Node {
	return gx.El("div", gx.Attrs{{Key: "role", Value: "menu"}, {Key: "class", Value: "w-56"}},
		ContextMenuSub(ContextMenuSubProps{Children: gx.Frag(
			ContextMenuSubTrigger(ContextMenuSubTriggerProps{Children: gx.Text("More tools")}),
			n,
		)}),
	)
}
