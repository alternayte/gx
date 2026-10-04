package toggle

import "github.com/alternayte/gx"

var ToggleFixtures = gx.Fixtures[ToggleProps]{
	"Off":      {Name: "bold", Children: gx.Text("Bold")},
	"On":       {Name: "bold", Pressed: true, Children: gx.Text("Bold")},
	"Disabled": {Name: "bold", Attrs: gx.Attrs{gx.Bool("disabled", true)}, Children: gx.Text("Bold")},
}
