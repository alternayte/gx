package contextmenu

import "github.com/alternayte/gx"

var ContextMenuLinkFixtures = gx.Fixtures[ContextMenuLinkProps]{"Link": {Href: gx.URL("/docs"), Children: gx.Text("Docs")}}

// ContextMenuLinkWrap renders the item inside a menu, as a page uses it.
func ContextMenuLinkWrap(n gx.Node) gx.Node {
	return gx.El("div", gx.Attrs{{Key: "role", Value: "menu"}}, n)
}
