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

var FieldLabelFixtures = gx.Fixtures[FieldLabelProps]{"Label": {For: "email", Children: gx.Text("Email")}}
var FieldDescriptionFixtures = gx.Fixtures[FieldDescriptionProps]{"Text": {Children: gx.Text("A short hint.")}}
var FieldErrorFixtures = gx.Fixtures[FieldErrorProps]{"Error": {Children: gx.Text("This field is required.")}}
var FieldGroupFixtures = gx.Fixtures[FieldGroupProps]{"Empty": {}}
var FieldSetFixtures = gx.Fixtures[FieldSetProps]{"Empty": {}}
var FieldLegendFixtures = gx.Fixtures[FieldLegendProps]{"Legend": {Children: gx.Text("Address")}}
