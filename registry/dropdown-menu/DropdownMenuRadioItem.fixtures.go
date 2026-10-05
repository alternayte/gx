package dropdownmenu

import "github.com/alternayte/gx"

var DropdownMenuRadioItemFixtures = gx.Fixtures[DropdownMenuRadioItemProps]{
	"Checked":   {Name: "dropdown-menu-item-checked", Value: "top", Checked: true, Children: gx.Text("Top")},
	"Unchecked": {Name: "dropdown-menu-item-unchecked", Value: "bottom", Children: gx.Text("Bottom")},
}

// DropdownMenuRadioItemWrap renders the item inside a menu, as a page uses it.
func DropdownMenuRadioItemWrap(n gx.Node) gx.Node {
	return gx.El("div", gx.Attrs{{Key: "role", Value: "menu"}, {Key: "class", Value: "w-56"}}, n)
}
