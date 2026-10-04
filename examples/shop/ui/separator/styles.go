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

// attrs returns the attributes of one separator.
func (p SeparatorProps) attrs() gx.Attrs {
	a := gx.Attrs{
		{Key: "role", Value: "separator"},
		{Key: "data-orientation", Value: string(p.Orientation)},
		{Key: "class", Value: gx.Cx("shrink-0 bg-border", orientationClass[p.Orientation], p.Class)},
	}
	if p.Decorative {
		a = append(a, gx.Attr{Key: "aria-hidden", Value: "true"})
	}
	return a
}
