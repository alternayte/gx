package dropdownmenu

import "github.com/alternayte/gx"

var DropdownMenuLinkFixtures = gx.Fixtures[DropdownMenuLinkProps]{"Link": {Href: gx.URL("/docs"), Children: gx.Text("Documentation")}}

// DropdownMenuLinkWrap renders the item inside a menu, as a page uses it.
func DropdownMenuLinkWrap(n gx.Node) gx.Node {
	return gx.El("div", gx.Attrs{{Key: "role", Value: "menu"}, {Key: "class", Value: "w-56"}}, n)
}
