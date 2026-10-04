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

// legendClass returns the classes of one legend variant; a zero value is
// Legend.
func (p FieldLegendProps) legendClass() string {
	if p.Variant == "" {
		return legendClass[Legend]
	}
	return legendClass[p.Variant]
}
