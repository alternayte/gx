package field

import "github.com/alternayte/gx"

var FieldFixtures = gx.Fixtures[FieldProps]{
	"LabelAndInput": {Children: gx.Frag(
		FieldLabel(FieldLabelProps{For: "email", Children: gx.Text("Email")}),
		FieldDescription(FieldDescriptionProps{Children: gx.Text("We never share your email.")}),
	)},
	"Invalid": {Invalid: true, Children: gx.Frag(
		FieldLabel(FieldLabelProps{For: "email", Children: gx.Text("Email")}),
		FieldError(FieldErrorProps{Children: gx.Text("Enter a valid email address.")}),
	)},
}
