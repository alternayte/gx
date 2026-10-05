package menubar

import "github.com/alternayte/gx"

var MenubarLabelFixtures = gx.Fixtures[MenubarLabelProps]{
	"Label": {Children: gx.Text("My account")},
	"Inset": {Inset: true, Children: gx.Text("My account")},
}
