package field

import "github.com/alternayte/gx"

// Variant is the size of a field legend.
type Variant string

// The variants of field.FieldLegend.
const (
	Legend Variant = "legend"
	Label  Variant = "label"
)

var legendClass = gx.Enum[Variant]{
	Legend: "text-base",
	Label:  "text-sm",
}
