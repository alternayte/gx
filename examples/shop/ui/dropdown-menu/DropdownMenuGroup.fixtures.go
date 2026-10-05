package dropdownmenu

import "github.com/alternayte/gx"

var DropdownMenuGroupFixtures = gx.Fixtures[DropdownMenuGroupProps]{
	"Default": {Label: "Account", Children: gx.Frag(
		DropdownMenuItem(DropdownMenuItemProps{Children: gx.Text("Profile")}),
		DropdownMenuItem(DropdownMenuItemProps{Children: gx.Text("Billing")}),
	)},
}

// DropdownMenuGroupWrap renders the group inside a menu, as a page uses it.
func DropdownMenuGroupWrap(n gx.Node) gx.Node {
	return gx.El("div", gx.Attrs{{Key: "role", Value: "menu"}, {Key: "class", Value: "w-56"}}, n)
}
