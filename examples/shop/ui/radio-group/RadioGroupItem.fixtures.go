package radiogroup

import "github.com/alternayte/gx"

var RadioGroupItemFixtures = gx.Fixtures[RadioGroupItemProps]{
	"Unchecked": {Name: "plan", Value: "free", Label: "Free"},
	"Checked":   {Name: "plan", Value: "pro", Checked: true, Label: "Pro"},
}
