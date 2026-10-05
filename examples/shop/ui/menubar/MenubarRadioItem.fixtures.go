package menubar

import "github.com/alternayte/gx"

var MenubarRadioItemFixtures = gx.Fixtures[MenubarRadioItemProps]{
	"Checked":   {Name: "menubar-item-checked", Value: "top", Checked: true, Children: gx.Text("Top")},
	"Unchecked": {Name: "menubar-item-unchecked", Value: "bottom", Children: gx.Text("Bottom")},
}

// MenubarRadioItemWrap renders the item inside a menu, as a page uses it.
func MenubarRadioItemWrap(n gx.Node) gx.Node {
	return gx.El("div", gx.Attrs{{Key: "role", Value: "menu"}, {Key: "class", Value: "w-56"}}, n)
}
