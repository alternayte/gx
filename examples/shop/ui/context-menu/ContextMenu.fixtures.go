package contextmenu

import "github.com/alternayte/gx"

var ContextMenuFixtures = gx.Fixtures[ContextMenuProps]{
	"Menu": {Id: "demo-context", Class: "w-52", Children: gx.Frag(
		ContextMenuItem(ContextMenuItemProps{Children: gx.Frag(
			gx.Text("Copy"),
			ContextMenuShortcut(ContextMenuShortcutProps{Children: gx.Text("⌘C")}),
		)}),
		ContextMenuItem(ContextMenuItemProps{Children: gx.Text("Cut")}),
		ContextMenuSeparator(ContextMenuSeparatorProps{}),
		ContextMenuCheckboxItem(ContextMenuCheckboxItemProps{Name: "bookmarks", Checked: true, Children: gx.Text("Show bookmarks")}),
		ContextMenuSeparator(ContextMenuSeparatorProps{}),
		ContextMenuRadioGroup(ContextMenuRadioGroupProps{Label: "People", Children: gx.Frag(
			ContextMenuLabel(ContextMenuLabelProps{Inset: true, Children: gx.Text("People")}),
			ContextMenuRadioItem(ContextMenuRadioItemProps{Name: "demo-context-person", Value: "ada", Checked: true, Children: gx.Text("Ada")}),
			ContextMenuRadioItem(ContextMenuRadioItemProps{Name: "demo-context-person", Value: "grace", Children: gx.Text("Grace")}),
		)}),
		ContextMenuSeparator(ContextMenuSeparatorProps{}),
		ContextMenuLink(ContextMenuLinkProps{Href: gx.URL("/docs"), Children: gx.Text("Docs")}),
		ContextMenuItem(ContextMenuItemProps{Variant: Destructive, Children: gx.Text("Delete")}),
	)},
	"Sub": {Id: "demo-context-sub", Class: "w-52", Children: gx.Frag(
		ContextMenuItem(ContextMenuItemProps{Children: gx.Text("Back")}),
		ContextMenuSub(ContextMenuSubProps{Children: gx.Frag(
			ContextMenuSubTrigger(ContextMenuSubTriggerProps{Children: gx.Text("More tools")}),
			ContextMenuSubContent(ContextMenuSubContentProps{Class: "w-48", Children: gx.Frag(
				ContextMenuItem(ContextMenuItemProps{Children: gx.Text("Save page")}),
				ContextMenuItem(ContextMenuItemProps{Children: gx.Text("Create shortcut")}),
				ContextMenuSeparator(ContextMenuSeparatorProps{}),
				ContextMenuItem(ContextMenuItemProps{Children: gx.Text("Developer tools")}),
			)}),
		)}),
		ContextMenuSeparator(ContextMenuSeparatorProps{}),
		ContextMenuItem(ContextMenuItemProps{Children: gx.Text("Reload")}),
	)},
}
