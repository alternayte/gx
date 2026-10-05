package menubar

import "github.com/alternayte/gx"

var MenubarMenuFixtures = gx.Fixtures[MenubarMenuProps]{
	"Menu": {Id: "demo-menubar-menu", Label: "Edit", Children: gx.Frag(
		MenubarItem(MenubarItemProps{Children: gx.Text("Undo")}),
		MenubarItem(MenubarItemProps{Children: gx.Text("Redo")}),
	)},
}

// MenubarMenuWrap renders the menu inside a bar, as a page uses it.
func MenubarMenuWrap(n gx.Node) gx.Node {
	return Menubar(MenubarProps{Children: n})
}
