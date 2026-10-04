package dropdownmenu

import "github.com/alternayte/gx"

var DropdownMenuFixtures = gx.Fixtures[DropdownMenuProps]{
	"Menu": {Id: "demo-dropdown", Children: gx.Frag(
		DropdownMenuLabel(DropdownMenuLabelProps{Children: gx.Text("My account")}),
		DropdownMenuSeparator(DropdownMenuSeparatorProps{}),
		DropdownMenuItem(DropdownMenuItemProps{Children: gx.Text("Profile")}),
		DropdownMenuLink(DropdownMenuLinkProps{Href: gx.URL("/docs"), Children: gx.Text("Documentation")}),
		DropdownMenuSeparator(DropdownMenuSeparatorProps{}),
		DropdownMenuItem(DropdownMenuItemProps{Children: gx.Text("Sign out")}),
	)},
}
