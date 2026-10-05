package menubar

import "github.com/alternayte/gx"

var MenubarSubContentFixtures = gx.Fixtures[MenubarSubContentProps]{
	"Content": {Class: "w-48", Children: gx.Frag(
		MenubarItem(MenubarItemProps{Children: gx.Text("Save page")}),
		MenubarItem(MenubarItemProps{Children: gx.Text("Create shortcut")}),
		MenubarSeparator(MenubarSeparatorProps{}),
		MenubarItem(MenubarItemProps{Children: gx.Text("Developer tools")}),
	)},
}

// MenubarSubContentWrap renders the content behind its trigger inside a menu,
// as a page uses it.
func MenubarSubContentWrap(n gx.Node) gx.Node {
	return gx.El("div", gx.Attrs{{Key: "role", Value: "menu"}, {Key: "class", Value: "w-56"}},
		MenubarSub(MenubarSubProps{Children: gx.Frag(
			MenubarSubTrigger(MenubarSubTriggerProps{Children: gx.Text("More tools")}),
			n,
		)}),
	)
}
