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
