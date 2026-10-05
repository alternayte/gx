package menubar

import "github.com/alternayte/gx"

var MenubarLinkFixtures = gx.Fixtures[MenubarLinkProps]{"Link": {Href: gx.URL("/docs"), Children: gx.Text("Documentation")}}

// MenubarLinkWrap renders the item inside a menu, as a page uses it.
func MenubarLinkWrap(n gx.Node) gx.Node {
	return gx.El("div", gx.Attrs{{Key: "role", Value: "menu"}, {Key: "class", Value: "w-56"}}, n)
}
