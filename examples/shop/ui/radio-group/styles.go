package radiogroup

import "github.com/alternayte/gx"

// Orientation is the layout direction of a radio group.
type Orientation string

// The orientations of radiogroup.RadioGroup.
const (
	Vertical   Orientation = "vertical"
	Horizontal Orientation = "horizontal"
)

var orientationClass = gx.Enum[Orientation]{
	Vertical:   "grid-cols-1",
	Horizontal: "grid-flow-col auto-cols-max gap-4",
}

// orientation returns the orientation of one group; a zero value is
// Vertical.
func (p RadioGroupProps) orientation() Orientation {
	if p.Orientation == "" {
		return Vertical
	}
	return p.Orientation
}

// invalid returns the aria-invalid value of the input.
func (p RadioGroupItemProps) invalid() string {
	if p.Invalid {
		return "true"
	}
	return "false"
}
