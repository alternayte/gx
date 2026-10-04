package kbd

import "github.com/alternayte/gx"

var KbdFixtures = gx.Fixtures[KbdProps]{
	"Key":   {Children: gx.Text("K")},
	"Combo": {Children: gx.Text("Ctrl")},
}
