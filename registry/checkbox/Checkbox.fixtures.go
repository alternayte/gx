package checkbox

import "github.com/alternayte/gx"

var CheckboxFixtures = gx.Fixtures[CheckboxProps]{
	"Unchecked":       {Name: "terms", Children: gx.Text("Accept the terms")},
	"Checked":         {Name: "terms", Checked: true, Children: gx.Text("Accept the terms")},
	"Disabled":        {Name: "terms", Disabled: true, Children: gx.Text("Disabled")},
	"DisabledChecked": {Name: "terms", Checked: true, Disabled: true, Children: gx.Text("Disabled")},
	"Invalid":         {Name: "terms", Invalid: true, Children: gx.Text("Accept the terms")},
}
