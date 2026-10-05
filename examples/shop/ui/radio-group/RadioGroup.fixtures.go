package radiogroup

import "github.com/alternayte/gx"

// Each fixture has its own name: radios that share a name form one group
// across the whole gallery page, and only one of them stays checked.
var RadioGroupFixtures = gx.Fixtures[RadioGroupProps]{
	"Vertical": {
		Name:  "plan",
		Label: "Plan",
		Children: gx.Frag(
			RadioGroupItem(RadioGroupItemProps{Name: "plan", Value: "free", Checked: true, Label: "Free"}),
			RadioGroupItem(RadioGroupItemProps{Name: "plan", Value: "pro", Label: "Pro"}),
		),
	},
	"Horizontal": {
		Name:        "size",
		Label:       "Size",
		Orientation: Horizontal,
		Children: gx.Frag(
			RadioGroupItem(RadioGroupItemProps{Name: "size", Value: "s", Checked: true, Label: "Small"}),
			RadioGroupItem(RadioGroupItemProps{Name: "size", Value: "m", Label: "Medium"}),
		),
	},
}
