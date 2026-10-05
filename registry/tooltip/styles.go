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

// The content sits 5px off the trigger, the height of the arrow. It zooms
// from the arrow and slides 2 units from the trigger side. round() keeps the
// centred content on whole pixels, so its text stays sharp.
var sideClass = gx.Enum[Side]{
	Top:    "bottom-full left-[round(50%,1px)] mb-[5px] origin-bottom -translate-x-[round(50%,1px)] translate-y-2 group-hover/tooltip:translate-y-0 group-has-[:focus-visible]/tooltip:translate-y-0",
	Bottom: "top-full left-[round(50%,1px)] mt-[5px] origin-top -translate-x-[round(50%,1px)] -translate-y-2 group-hover/tooltip:translate-y-0 group-has-[:focus-visible]/tooltip:translate-y-0",
	Left:   "right-full top-[round(50%,1px)] mr-[5px] origin-right -translate-y-[round(50%,1px)] translate-x-2 group-hover/tooltip:translate-x-0 group-has-[:focus-visible]/tooltip:translate-x-0",
	Right:  "left-full top-[round(50%,1px)] ml-[5px] origin-left -translate-y-[round(50%,1px)] -translate-x-2 group-hover/tooltip:translate-x-0 group-has-[:focus-visible]/tooltip:translate-x-0",
}

// The arrow is a rotated square whose centre lies 2px inside the edge of
// the content, as the reference draws it.
var arrowClass = gx.Enum[Side]{
	Top:    "top-full left-[round(50%,1px)] -translate-x-1/2 -translate-y-[calc(50%+2px)]",
	Bottom: "bottom-full left-[round(50%,1px)] -translate-x-1/2 translate-y-[calc(50%+2px)]",
	Left:   "left-full top-[round(50%,1px)] -translate-x-[calc(50%+2px)] -translate-y-1/2",
	Right:  "right-full top-[round(50%,1px)] translate-x-[calc(50%+2px)] -translate-y-1/2",
}

// side returns the side of one tooltip; a zero value is Top.
func (p TooltipProps) side() Side {
	if p.Side == "" {
		return Top
	}
	return p.Side
}
