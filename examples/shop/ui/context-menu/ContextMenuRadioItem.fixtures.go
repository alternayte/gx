package contextmenu

import "github.com/alternayte/gx"

var ContextMenuRadioItemFixtures = gx.Fixtures[ContextMenuRadioItemProps]{
	"Checked":   {Name: "context-menu-item-checked", Value: "top", Checked: true, Children: gx.Text("Top")},
	"Unchecked": {Name: "context-menu-item-unchecked", Value: "bottom", Children: gx.Text("Bottom")},
}

// ContextMenuRadioItemWrap renders the item inside a menu, as a page uses it.
func ContextMenuRadioItemWrap(n gx.Node) gx.Node {
	return gx.El("div", gx.Attrs{{Key: "role", Value: "menu"}, {Key: "class", Value: "w-56"}}, n)
}
