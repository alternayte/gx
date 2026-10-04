package contextmenu

import "github.com/alternayte/gx"

var ContextMenuItemFixtures = gx.Fixtures[ContextMenuItemProps]{"Item": {Children: gx.Text("Copy")}}

// ContextMenuItemWrap renders the item inside a menu, as a page uses it.
func ContextMenuItemWrap(n gx.Node) gx.Node {
	return gx.El("div", gx.Attrs{{Key: "role", Value: "menu"}}, n)
}
