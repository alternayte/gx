package togglegroup

import "github.com/alternayte/gx"

var ToggleGroupFixtures = gx.Fixtures[ToggleGroupProps]{
	"Three": {Children: gx.Frag(
		ToggleGroupItem(ToggleGroupItemProps{Name: "align", Value: "left", Checked: true, Children: gx.Text("Left")}),
		ToggleGroupItem(ToggleGroupItemProps{Name: "align", Value: "center", Children: gx.Text("Center")}),
		ToggleGroupItem(ToggleGroupItemProps{Name: "align", Value: "right", Children: gx.Text("Right")}),
	)},
}
