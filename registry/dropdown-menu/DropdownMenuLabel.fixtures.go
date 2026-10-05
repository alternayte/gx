package dropdownmenu

import "github.com/alternayte/gx"

var DropdownMenuLabelFixtures = gx.Fixtures[DropdownMenuLabelProps]{
	"Label": {Children: gx.Text("My account")},
	"Inset": {Inset: true, Children: gx.Text("My account")},
}
