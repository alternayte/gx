package field

import (
	"github.com/alternayte/gx"
	"github.com/alternayte/gx/registry/input"
)

var FieldFixtures = gx.Fixtures[FieldProps]{
	"LabelAndInput": {Children: gx.Frag(
		FieldLabel(FieldLabelProps{For: "field-email", Children: gx.Text("Email")}),
		input.Input(input.InputProps{Id: "field-email", Type: "email", Placeholder: "you@example.com"}),
		FieldDescription(FieldDescriptionProps{Children: gx.Text("We never share your email.")}),
	)},
	"Invalid": {Invalid: true, Children: gx.Frag(
		FieldLabel(FieldLabelProps{For: "field-invalid", Children: gx.Text("Email")}),
		input.Input(input.InputProps{Id: "field-invalid", Type: "email", Value: "ada@", Attrs: gx.Attrs{{Key: "aria-invalid", Value: "true"}}}),
		FieldError(FieldErrorProps{Children: gx.Text("Enter a valid email address.")}),
	)},
	"Disabled": {Disabled: true, Children: gx.Frag(
		FieldLabel(FieldLabelProps{For: "field-disabled", Children: gx.Text("Email")}),
		input.Input(input.InputProps{Id: "field-disabled", Type: "email", Attrs: gx.Attrs{gx.Bool("disabled", true)}}),
	)},
	"Horizontal": {Orientation: Horizontal, Children: gx.Frag(
		FieldContent(FieldContentProps{Children: gx.Frag(
			FieldTitle(FieldTitleProps{Children: gx.Text("Product news")}),
			FieldDescription(FieldDescriptionProps{Children: gx.Text("One message each month.")}),
		)}),
		gx.El("input", gx.Attrs{
			{Key: "type", Value: "checkbox"},
			{Key: "aria-label", Value: "Product news"},
			{Key: "class", Value: "size-4 accent-primary"},
		}),
	)},
	"Responsive": {Orientation: Responsive, Children: gx.Frag(
		FieldLabel(FieldLabelProps{For: "field-responsive", Children: gx.Text("Name")}),
		input.Input(input.InputProps{Id: "field-responsive"}),
	)},
}

// FieldWrap renders the field in a field group, as a form uses it. The
// group is the container that a responsive field measures.
func FieldWrap(n gx.Node) gx.Node {
	return FieldGroup(FieldGroupProps{Children: n})
}
