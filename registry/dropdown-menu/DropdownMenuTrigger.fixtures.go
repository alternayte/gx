package dropdownmenu

import (
	"github.com/alternayte/gx"
	"github.com/alternayte/gx/registry/button"
)

var DropdownMenuTriggerFixtures = gx.Fixtures[DropdownMenuTriggerProps]{
	"Default": {Id: "demo-dropdown", Children: gx.Text("Open menu")},
	"Ghost":   {Id: "demo-dropdown-end", Variant: button.Ghost, Size: button.Sm, Class: "ml-32", Children: gx.Text("Open at the end")},
	"Start":   {Id: "demo-dropdown-start", Variant: button.Secondary, Children: gx.Text("Open at the start")},
}
