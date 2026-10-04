package radiogroup

import "github.com/alternayte/gx"

var RadioGroupFixtures = gx.Fixtures[RadioGroupProps]{
	"Vertical": {
		Name: "plan",
		Children: gx.Frag(
			RadioGroupItem(RadioGroupItemProps{Name: "plan", Value: "free", Checked: true, Label: "Free"}),
			RadioGroupItem(RadioGroupItemProps{Name: "plan", Value: "pro", Label: "Pro"}),
		),
	},
	"Horizontal": {
		Name:        "size",
		Orientation: Horizontal,
		Children: gx.Frag(
			RadioGroupItem(RadioGroupItemProps{Name: "size", Value: "s", Checked: true, Label: "Small"}),
			RadioGroupItem(RadioGroupItemProps{Name: "size", Value: "m", Label: "Medium"}),
		),
	},
}
