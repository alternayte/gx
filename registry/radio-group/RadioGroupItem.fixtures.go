package radiogroup

import "github.com/alternayte/gx"

var RadioGroupItemFixtures = gx.Fixtures[RadioGroupItemProps]{
	"Unchecked": {Name: "item-unchecked", Value: "free", Label: "Free"},
	"Checked":   {Name: "item-checked", Value: "pro", Checked: true, Label: "Pro"},
	"Disabled":  {Name: "item-disabled", Value: "pro", Checked: true, Disabled: true, Label: "Pro"},
	"Invalid":   {Name: "item-invalid", Value: "pro", Invalid: true, Label: "Pro"},
}
