package field

import (
	"github.com/alternayte/gx"
	"github.com/alternayte/gx/registry/input"
)

var FieldSetFixtures = gx.Fixtures[FieldSetProps]{
	"Address": {Children: gx.Frag(
		FieldLegend(FieldLegendProps{Children: gx.Text("Address")}),
		FieldDescription(FieldDescriptionProps{Children: gx.Text("We send the invoice to this address.")}),
		FieldGroup(FieldGroupProps{Children: gx.Frag(
			Field(FieldProps{Children: gx.Frag(
				FieldLabel(FieldLabelProps{For: "set-street", Children: gx.Text("Street")}),
				input.Input(input.InputProps{Id: "set-street"}),
			)}),
			Field(FieldProps{Children: gx.Frag(
				FieldLabel(FieldLabelProps{For: "set-city", Children: gx.Text("City")}),
				input.Input(input.InputProps{Id: "set-city"}),
			)}),
		)}),
	)},
}
