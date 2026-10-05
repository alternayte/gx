package field

import (
	"github.com/alternayte/gx"
	"github.com/alternayte/gx/examples/shop/ui/input"
)

var FieldGroupFixtures = gx.Fixtures[FieldGroupProps]{
	"TwoFields": {Children: gx.Frag(
		Field(FieldProps{Children: gx.Frag(
			FieldLabel(FieldLabelProps{For: "group-name", Children: gx.Text("Name")}),
			input.Input(input.InputProps{Id: "group-name"}),
		)}),
		FieldSeparator(FieldSeparatorProps{}),
		Field(FieldProps{Children: gx.Frag(
			FieldLabel(FieldLabelProps{For: "group-email", Children: gx.Text("Email")}),
			input.Input(input.InputProps{Id: "group-email", Type: "email"}),
		)}),
	)},
}
