package contextmenu

import "github.com/alternayte/gx"

var ContextMenuFixtures = gx.Fixtures[ContextMenuProps]{
	"Menu": {Id: "demo-context", Children: gx.Frag(
		ContextMenuItem(ContextMenuItemProps{Children: gx.Text("Copy")}),
		ContextMenuItem(ContextMenuItemProps{Children: gx.Text("Cut")}),
		ContextMenuSeparator(ContextMenuSeparatorProps{}),
		ContextMenuLink(ContextMenuLinkProps{Href: gx.URL("/docs"), Children: gx.Text("Docs")}),
	)},
}
