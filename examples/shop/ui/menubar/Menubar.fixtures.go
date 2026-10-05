package menubar

import "github.com/alternayte/gx"

var MenubarFixtures = gx.Fixtures[MenubarProps]{
	"Default": {Children: gx.Frag(
		MenubarMenu(MenubarMenuProps{Id: "demo-menubar-file", Label: "File", Children: gx.Frag(
			MenubarItem(MenubarItemProps{Children: gx.Frag(
				gx.Text("New tab"),
				MenubarShortcut(MenubarShortcutProps{Children: gx.Text("⌘T")}),
			)}),
			MenubarItem(MenubarItemProps{Children: gx.Text("New window")}),
			MenubarSeparator(MenubarSeparatorProps{}),
			MenubarLink(MenubarLinkProps{Href: gx.URL("/docs"), Children: gx.Text("Documentation")}),
		)}),
		MenubarMenu(MenubarMenuProps{Id: "demo-menubar-view", Label: "View", Children: gx.Frag(
			MenubarCheckboxItem(MenubarCheckboxItemProps{Name: "bookmarks", Children: gx.Text("Always show bookmarks bar")}),
			MenubarCheckboxItem(MenubarCheckboxItemProps{Name: "urls", Checked: true, Children: gx.Text("Always show full URLs")}),
			MenubarSeparator(MenubarSeparatorProps{}),
			MenubarItem(MenubarItemProps{Inset: true, Children: gx.Frag(
				gx.Text("Reload"),
				MenubarShortcut(MenubarShortcutProps{Children: gx.Text("⌘R")}),
			)}),
			MenubarItem(MenubarItemProps{Inset: true, Disabled: true, Children: gx.Text("Force reload")}),
		)}),
		MenubarMenu(MenubarMenuProps{Id: "demo-menubar-profiles", Label: "Profiles", Children: gx.Frag(
			MenubarRadioGroup(MenubarRadioGroupProps{Label: "Profile", Children: gx.Frag(
				MenubarLabel(MenubarLabelProps{Inset: true, Children: gx.Text("Profile")}),
				MenubarRadioItem(MenubarRadioItemProps{Name: "demo-menubar-profile", Value: "ada", Children: gx.Text("Ada")}),
				MenubarRadioItem(MenubarRadioItemProps{Name: "demo-menubar-profile", Value: "grace", Checked: true, Children: gx.Text("Grace")}),
			)}),
			MenubarSeparator(MenubarSeparatorProps{}),
			MenubarItem(MenubarItemProps{Inset: true, Variant: Destructive, Children: gx.Text("Remove profile")}),
		)}),
	)},
}
