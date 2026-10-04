package checkbox

import "github.com/alternayte/gx"

var CheckboxFixtures = gx.Fixtures[CheckboxProps]{
	"Unchecked": {Name: "terms", Children: gx.Text("Accept the terms")},
	"Checked":   {Name: "terms", Checked: true, Children: gx.Text("Accept the terms")},
	"Disabled":  {Name: "terms", Attrs: gx.Attrs{gx.Bool("disabled", true)}, Children: gx.Text("Disabled")},
}
