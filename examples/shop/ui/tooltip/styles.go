package tooltip

import "github.com/alternayte/gx"

// Side is the edge a tooltip appears on.
type Side string

// The sides of tooltip.Tooltip.
const (
	Top    Side = "top"
	Bottom Side = "bottom"
	Left   Side = "left"
	Right  Side = "right"
)

var sideClass = gx.Enum[Side]{
	Top:    "bottom-full left-1/2 mb-1.5 -translate-x-1/2",
	Bottom: "top-full left-1/2 mt-1.5 -translate-x-1/2",
	Left:   "right-full top-1/2 mr-1.5 -translate-y-1/2",
	Right:  "left-full top-1/2 ml-1.5 -translate-y-1/2",
}

// sideClass returns the classes of one tooltip side; a zero value is Top.
func (p TooltipProps) sideClass() string {
	if p.Side == "" {
		return sideClass[Top]
	}
	return sideClass[p.Side]
}
