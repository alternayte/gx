package dropdownmenu

import "github.com/alternayte/gx"

var DropdownMenuFixtures = gx.Fixtures[DropdownMenuProps]{
	"Menu": {Id: "demo-dropdown", Class: "w-56", Children: gx.Frag(
		DropdownMenuLabel(DropdownMenuLabelProps{Children: gx.Text("My account")}),
		DropdownMenuSeparator(DropdownMenuSeparatorProps{}),
		DropdownMenuGroup(DropdownMenuGroupProps{Label: "Account", Children: gx.Frag(
			DropdownMenuItem(DropdownMenuItemProps{Children: gx.Text("Profile")}),
			DropdownMenuItem(DropdownMenuItemProps{Children: gx.Frag(
				gx.Text("Settings"),
				DropdownMenuShortcut(DropdownMenuShortcutProps{Children: gx.Text("⌘S")}),
			)}),
			DropdownMenuLink(DropdownMenuLinkProps{Href: gx.URL("/docs"), Children: gx.Text("Documentation")}),
		)}),
		DropdownMenuSeparator(DropdownMenuSeparatorProps{}),
		DropdownMenuCheckboxItem(DropdownMenuCheckboxItemProps{Name: "status-bar", Checked: true, Children: gx.Text("Status bar")}),
		DropdownMenuCheckboxItem(DropdownMenuCheckboxItemProps{Name: "panel", Children: gx.Text("Panel")}),
		DropdownMenuSeparator(DropdownMenuSeparatorProps{}),
		DropdownMenuRadioGroup(DropdownMenuRadioGroupProps{Label: "Position", Children: gx.Frag(
			DropdownMenuRadioItem(DropdownMenuRadioItemProps{Name: "demo-dropdown-position", Value: "top", Checked: true, Children: gx.Text("Top")}),
			DropdownMenuRadioItem(DropdownMenuRadioItemProps{Name: "demo-dropdown-position", Value: "bottom", Children: gx.Text("Bottom")}),
		)}),
		DropdownMenuSeparator(DropdownMenuSeparatorProps{}),
		DropdownMenuItem(DropdownMenuItemProps{Variant: Destructive, Children: gx.Text("Sign out")}),
	)},
	"End": {Id: "demo-dropdown-end", Align: End, Children: gx.Frag(
		DropdownMenuItem(DropdownMenuItemProps{Children: gx.Text("Rename")}),
		DropdownMenuItem(DropdownMenuItemProps{Children: gx.Text("Duplicate")}),
		DropdownMenuItem(DropdownMenuItemProps{Disabled: true, Children: gx.Text("Archive")}),
		DropdownMenuItem(DropdownMenuItemProps{Variant: Destructive, Children: gx.Text("Delete")}),
	)},
	"Start": {Id: "demo-dropdown-start", Align: Start, Children: gx.Frag(
		DropdownMenuItem(DropdownMenuItemProps{Children: gx.Text("Rename")}),
		DropdownMenuItem(DropdownMenuItemProps{Children: gx.Text("Duplicate")}),
	)},
	"Sub": {Id: "demo-dropdown-sub", Class: "w-56", Children: gx.Frag(
		DropdownMenuItem(DropdownMenuItemProps{Children: gx.Text("New tab")}),
		DropdownMenuSub(DropdownMenuSubProps{Children: gx.Frag(
			DropdownMenuSubTrigger(DropdownMenuSubTriggerProps{Children: gx.Text("More tools")}),
			DropdownMenuSubContent(DropdownMenuSubContentProps{Class: "w-48", Children: gx.Frag(
				DropdownMenuItem(DropdownMenuItemProps{Children: gx.Text("Save page")}),
				DropdownMenuItem(DropdownMenuItemProps{Children: gx.Text("Create shortcut")}),
				DropdownMenuSeparator(DropdownMenuSeparatorProps{}),
				DropdownMenuItem(DropdownMenuItemProps{Children: gx.Text("Developer tools")}),
			)}),
		)}),
		DropdownMenuSeparator(DropdownMenuSeparatorProps{}),
		DropdownMenuItem(DropdownMenuItemProps{Children: gx.Text("Print")}),
	)},
}
