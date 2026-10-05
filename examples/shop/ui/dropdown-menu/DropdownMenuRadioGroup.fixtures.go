package dropdownmenu

import "github.com/alternayte/gx"

var DropdownMenuRadioGroupFixtures = gx.Fixtures[DropdownMenuRadioGroupProps]{
	"Default": {Label: "Position", Children: gx.Frag(
		DropdownMenuRadioItem(DropdownMenuRadioItemProps{Name: "dropdown-menu-group-position", Value: "top", Checked: true, Children: gx.Text("Top")}),
		DropdownMenuRadioItem(DropdownMenuRadioItemProps{Name: "dropdown-menu-group-position", Value: "bottom", Children: gx.Text("Bottom")}),
	)},
}

// DropdownMenuRadioGroupWrap renders the group inside a menu, as a page uses it.
func DropdownMenuRadioGroupWrap(n gx.Node) gx.Node {
	return gx.El("div", gx.Attrs{{Key: "role", Value: "menu"}, {Key: "class", Value: "w-56"}}, n)
}
