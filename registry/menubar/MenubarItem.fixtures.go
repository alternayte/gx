package menubar

import "github.com/alternayte/gx"

var MenubarItemFixtures = gx.Fixtures[MenubarItemProps]{"Item": {Href: gx.URL("/docs"), Children: gx.Text("Docs")}}

// MenubarItemWrap renders the item inside a bar, as a page uses it.
func MenubarItemWrap(n gx.Node) gx.Node {
	return Menubar(MenubarProps{Children: n})
}
