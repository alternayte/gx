package menubar

import "github.com/alternayte/gx"

var MenubarSubFixtures = gx.Fixtures[MenubarSubProps]{
	"Default": {Children: gx.Frag(
		MenubarSubTrigger(MenubarSubTriggerProps{Children: gx.Text("More tools")}),
		MenubarSubContent(MenubarSubContentProps{Children: gx.Frag(
			MenubarItem(MenubarItemProps{Children: gx.Text("Save page")}),
			MenubarItem(MenubarItemProps{Children: gx.Text("Create shortcut")}),
		)}),
	)},
}

// MenubarSubWrap renders the sub-menu inside a menu, as a page uses it.
func MenubarSubWrap(n gx.Node) gx.Node {
	return gx.El("div", gx.Attrs{{Key: "role", Value: "menu"}, {Key: "class", Value: "w-56"}}, n)
}
