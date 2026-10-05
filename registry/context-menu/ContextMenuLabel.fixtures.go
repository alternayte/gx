package contextmenu

import "github.com/alternayte/gx"

var ContextMenuLabelFixtures = gx.Fixtures[ContextMenuLabelProps]{
	"Label": {Children: gx.Text("My account")},
	"Inset": {Inset: true, Children: gx.Text("My account")},
}
