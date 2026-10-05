package separator

import "github.com/alternayte/gx"

// Orientation is the direction of a separator.
type Orientation string

// The orientations of separator.Separator.
const (
	Horizontal Orientation = "horizontal"
	Vertical   Orientation = "vertical"
)

var orientationClass = gx.Enum[Orientation]{
	Horizontal: "h-px w-full",
	Vertical:   "h-full w-px",
}

// orientation returns the data-orientation value; a zero value is
// Horizontal.
func (p SeparatorProps) orientation() string {
	if p.Orientation == "" {
		return string(Horizontal)
	}
	return string(p.Orientation)
}

// attrs returns the role of one separator, then the caller's attributes. A
// decorative separator has no role. A semantic separator names its
// orientation when it is vertical.
func (p SeparatorProps) attrs() gx.Attrs {
	a := gx.Attrs{{Key: "role", Value: "none"}}
	if !p.Decorative {
		a = gx.Attrs{{Key: "role", Value: "separator"}}
		if p.Orientation == Vertical {
			a = append(a, gx.Attr{Key: "aria-orientation", Value: "vertical"})
		}
	}
	return append(a, p.Attrs...)
}
