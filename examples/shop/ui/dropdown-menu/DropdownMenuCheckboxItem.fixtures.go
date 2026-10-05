package dropdownmenu

import "github.com/alternayte/gx"

var DropdownMenuCheckboxItemFixtures = gx.Fixtures[DropdownMenuCheckboxItemProps]{
	"Checked":   {Name: "status-bar", Checked: true, Children: gx.Text("Status bar")},
	"Unchecked": {Name: "panel", Children: gx.Text("Panel")},
	"Disabled":  {Name: "activity", Disabled: true, Children: gx.Text("Activity bar")},
}

// DropdownMenuCheckboxItemWrap renders the item inside a menu, as a page uses it.
func DropdownMenuCheckboxItemWrap(n gx.Node) gx.Node {
	return gx.El("div", gx.Attrs{{Key: "role", Value: "menu"}, {Key: "class", Value: "w-56"}}, n)
}
