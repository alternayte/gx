package contextmenu

import "github.com/alternayte/gx"

var ContextMenuItemFixtures = gx.Fixtures[ContextMenuItemProps]{
	"Item":        {Children: gx.Text("Profile")},
	"Inset":       {Inset: true, Children: gx.Text("Profile")},
	"Destructive": {Variant: Destructive, Children: gx.Text("Delete")},
	"Disabled":    {Disabled: true, Children: gx.Text("Profile")},
}

// ContextMenuItemWrap renders the item inside a menu, as a page uses it.
func ContextMenuItemWrap(n gx.Node) gx.Node {
	return gx.El("div", gx.Attrs{{Key: "role", Value: "menu"}, {Key: "class", Value: "w-56"}}, n)
}
