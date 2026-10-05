package buttongroup

import "github.com/alternayte/gx"

// Orientation is the direction of a button group or of its separator.
type Orientation string

// The orientations of buttongroup.ButtonGroup and
// buttongroup.ButtonGroupSeparator.
const (
	Horizontal Orientation = "horizontal"
	Vertical   Orientation = "vertical"
)

var orientationClass = gx.Enum[Orientation]{
	Horizontal: "[&>*:not(:first-child)]:rounded-l-none [&>*:not(:first-child)]:border-l-0 [&>*:not(:last-child)]:rounded-r-none",
	Vertical:   "flex-col [&>*:not(:first-child)]:rounded-t-none [&>*:not(:first-child)]:border-t-0 [&>*:not(:last-child)]:rounded-b-none",
}

// separatorClass sizes the separator: a vertical line takes the height of
// the group, a horizontal line takes its width.
var separatorClass = gx.Enum[Orientation]{
	Horizontal: "h-px w-full",
	Vertical:   "h-auto w-px",
}

// orientation returns the data-orientation value; a zero value is
// Horizontal.
func (p ButtonGroupProps) orientation() string {
	if p.Orientation == "" {
		return string(Horizontal)
	}
	return string(p.Orientation)
}

// orientation returns the data-orientation value; a zero value is
// Vertical, the line between the buttons of a horizontal group.
func (p ButtonGroupSeparatorProps) orientation() string {
	if p.Orientation == "" {
		return string(Vertical)
	}
	return string(p.Orientation)
}
