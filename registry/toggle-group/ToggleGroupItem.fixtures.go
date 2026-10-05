package togglegroup

import "github.com/alternayte/gx"

var ToggleGroupItemFixtures = gx.Fixtures[ToggleGroupItemProps]{
	"Checked":  {Name: "align-demo", Value: "left", Checked: true, Children: gx.Text("Left")},
	"Disabled": {Name: "align-disabled", Value: "left", Disabled: true, Children: gx.Text("Left")},
}

// ToggleGroupItemWrap renders the item inside a group, as a page uses it.
func ToggleGroupItemWrap(n gx.Node) gx.Node {
	return ToggleGroup(ToggleGroupProps{Label: "Alignment", Children: n})
}
