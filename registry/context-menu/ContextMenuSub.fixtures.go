package contextmenu

import "github.com/alternayte/gx"

var ContextMenuSubFixtures = gx.Fixtures[ContextMenuSubProps]{
	"Default": {Children: gx.Frag(
		ContextMenuSubTrigger(ContextMenuSubTriggerProps{Children: gx.Text("More tools")}),
		ContextMenuSubContent(ContextMenuSubContentProps{Children: gx.Frag(
			ContextMenuItem(ContextMenuItemProps{Children: gx.Text("Save page")}),
			ContextMenuItem(ContextMenuItemProps{Children: gx.Text("Create shortcut")}),
		)}),
	)},
}

// ContextMenuSubWrap renders the sub-menu inside a menu, as a page uses it.
func ContextMenuSubWrap(n gx.Node) gx.Node {
	return gx.El("div", gx.Attrs{{Key: "role", Value: "menu"}, {Key: "class", Value: "w-56"}}, n)
}
