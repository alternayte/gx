package contextmenu

import "github.com/alternayte/gx"

var ContextMenuGroupFixtures = gx.Fixtures[ContextMenuGroupProps]{
	"Default": {Label: "Account", Children: gx.Frag(
		ContextMenuItem(ContextMenuItemProps{Children: gx.Text("Profile")}),
		ContextMenuItem(ContextMenuItemProps{Children: gx.Text("Billing")}),
	)},
}

// ContextMenuGroupWrap renders the group inside a menu, as a page uses it.
func ContextMenuGroupWrap(n gx.Node) gx.Node {
	return gx.El("div", gx.Attrs{{Key: "role", Value: "menu"}, {Key: "class", Value: "w-56"}}, n)
}
