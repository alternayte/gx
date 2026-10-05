package contextmenu

import "github.com/alternayte/gx"

var ContextMenuCheckboxItemFixtures = gx.Fixtures[ContextMenuCheckboxItemProps]{
	"Checked":   {Name: "status-bar", Checked: true, Children: gx.Text("Status bar")},
	"Unchecked": {Name: "panel", Children: gx.Text("Panel")},
	"Disabled":  {Name: "activity", Disabled: true, Children: gx.Text("Activity bar")},
}

// ContextMenuCheckboxItemWrap renders the item inside a menu, as a page uses it.
func ContextMenuCheckboxItemWrap(n gx.Node) gx.Node {
	return gx.El("div", gx.Attrs{{Key: "role", Value: "menu"}, {Key: "class", Value: "w-56"}}, n)
}
