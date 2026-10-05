package menubar

import "github.com/alternayte/gx"

var MenubarRadioGroupFixtures = gx.Fixtures[MenubarRadioGroupProps]{
	"Default": {Label: "Position", Children: gx.Frag(
		MenubarRadioItem(MenubarRadioItemProps{Name: "menubar-group-position", Value: "top", Checked: true, Children: gx.Text("Top")}),
		MenubarRadioItem(MenubarRadioItemProps{Name: "menubar-group-position", Value: "bottom", Children: gx.Text("Bottom")}),
	)},
}

// MenubarRadioGroupWrap renders the group inside a menu, as a page uses it.
func MenubarRadioGroupWrap(n gx.Node) gx.Node {
	return gx.El("div", gx.Attrs{{Key: "role", Value: "menu"}, {Key: "class", Value: "w-56"}}, n)
}
