package dropdownmenu

import "github.com/alternayte/gx"

var DropdownMenuItemFixtures = gx.Fixtures[DropdownMenuItemProps]{"Item": {Children: gx.Text("Profile")}}

// DropdownMenuItemWrap renders the item inside a menu, as a page uses it.
func DropdownMenuItemWrap(n gx.Node) gx.Node {
	return gx.El("div", gx.Attrs{{Key: "role", Value: "menu"}}, n)
}
