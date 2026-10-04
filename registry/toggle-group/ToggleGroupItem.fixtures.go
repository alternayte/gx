package togglegroup

import "github.com/alternayte/gx"

var ToggleGroupItemFixtures = gx.Fixtures[ToggleGroupItemProps]{
	"Checked": {Name: "align", Value: "left", Checked: true, Children: gx.Text("Left")},
}
