package menubar

import "github.com/alternayte/gx"

var MenubarGroupFixtures = gx.Fixtures[MenubarGroupProps]{
	"Default": {Label: "Account", Children: gx.Frag(
		MenubarItem(MenubarItemProps{Children: gx.Text("Profile")}),
		MenubarItem(MenubarItemProps{Children: gx.Text("Billing")}),
	)},
}

// MenubarGroupWrap renders the group inside a menu, as a page uses it.
func MenubarGroupWrap(n gx.Node) gx.Node {
	return gx.El("div", gx.Attrs{{Key: "role", Value: "menu"}, {Key: "class", Value: "w-56"}}, n)
}
