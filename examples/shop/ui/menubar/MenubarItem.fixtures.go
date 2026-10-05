package menubar

import "github.com/alternayte/gx"

var MenubarItemFixtures = gx.Fixtures[MenubarItemProps]{
	"Item":        {Children: gx.Text("Profile")},
	"Inset":       {Inset: true, Children: gx.Text("Profile")},
	"Destructive": {Variant: Destructive, Children: gx.Text("Delete")},
	"Disabled":    {Disabled: true, Children: gx.Text("Profile")},
}

// MenubarItemWrap renders the item inside a menu, as a page uses it.
func MenubarItemWrap(n gx.Node) gx.Node {
	return gx.El("div", gx.Attrs{{Key: "role", Value: "menu"}, {Key: "class", Value: "w-56"}}, n)
}
