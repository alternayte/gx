package field

import "github.com/alternayte/gx"

// Variant is the size of a field legend.
type Variant string

// The variants of field.FieldLegend.
const (
	Legend Variant = "legend"
	Label  Variant = "label"
)

// variant returns the data-variant value; a zero value is Legend. The
// legend size and the gap below it follow this attribute.
func (p FieldLegendProps) variant() string {
	if p.Variant == "" {
		return string(Legend)
	}
	return string(p.Variant)
}

// Orientation is the direction of the label and the control of a field.
type Orientation string

// The orientations of field.Field. Responsive is vertical in a narrow
// field group and horizontal in a wide one.
const (
	Vertical   Orientation = "vertical"
	Horizontal Orientation = "horizontal"
	Responsive Orientation = "responsive"
)

var orientationClass = gx.Enum[Orientation]{
	Vertical:   "flex-col [&>*]:w-full [&>.sr-only]:w-auto",
	Horizontal: "flex-row items-center [&>[data-slot=field-label]]:flex-auto has-[>[data-slot=field-content]]:items-start has-[>[data-slot=field-content]]:[&>:is([role=checkbox],[role=radio],input[type=checkbox],input[type=radio])]:mt-px",
	Responsive: "flex-col @md/field-group:flex-row @md/field-group:items-center [&>*]:w-full @md/field-group:[&>*]:w-auto [&>.sr-only]:w-auto @md/field-group:[&>[data-slot=field-label]]:flex-auto @md/field-group:has-[>[data-slot=field-content]]:items-start @md/field-group:has-[>[data-slot=field-content]]:[&>:is([role=checkbox],[role=radio],input[type=checkbox],input[type=radio])]:mt-px",
}

// orientation returns the data-orientation value; a zero value is Vertical.
func (p FieldProps) orientation() string {
	if p.Orientation == "" {
		return string(Vertical)
	}
	return string(p.Orientation)
}

// attrs returns the state attributes of one field, then the caller's. The
// field and its label read data-invalid and data-disabled.
func (p FieldProps) attrs() gx.Attrs {
	var a gx.Attrs
	if p.Invalid {
		a = append(a, gx.Attr{Key: "data-invalid", Value: "true"})
	}
	if p.Disabled {
		a = append(a, gx.Attr{Key: "data-disabled", Value: "true"})
	}
	return append(a, p.Attrs...)
}

// attrs returns the for attribute when the label names a control, then the
// caller's attributes.
func (p FieldLabelProps) attrs() gx.Attrs {
	if p.For == "" {
		return p.Attrs
	}
	return append(gx.Attrs{{Key: "for", Value: p.For}}, p.Attrs...)
}

// messages returns the distinct, non-empty errors in their first order.
func (p FieldErrorProps) messages() []string {
	seen := make(map[string]bool, len(p.Errors))
	out := make([]string, 0, len(p.Errors))
	for _, message := range p.Errors {
		if message == "" || seen[message] {
			continue
		}
		seen[message] = true
		out = append(out, message)
	}
	return out
}
