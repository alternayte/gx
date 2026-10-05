package contextmenu

import "github.com/alternayte/gx"

var ContextMenuRadioGroupFixtures = gx.Fixtures[ContextMenuRadioGroupProps]{
	"Default": {Label: "Position", Children: gx.Frag(
		ContextMenuRadioItem(ContextMenuRadioItemProps{Name: "context-menu-group-position", Value: "top", Checked: true, Children: gx.Text("Top")}),
		ContextMenuRadioItem(ContextMenuRadioItemProps{Name: "context-menu-group-position", Value: "bottom", Children: gx.Text("Bottom")}),
	)},
}

// ContextMenuRadioGroupWrap renders the group inside a menu, as a page uses it.
func ContextMenuRadioGroupWrap(n gx.Node) gx.Node {
	return gx.El("div", gx.Attrs{{Key: "role", Value: "menu"}, {Key: "class", Value: "w-56"}}, n)
}
