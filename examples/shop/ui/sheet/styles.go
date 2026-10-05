package sheet

import "github.com/alternayte/gx"

// Side is the edge a sheet slides from.
type Side string

// The sides of sheet.Sheet.
const (
	Right  Side = "right"
	Left   Side = "left"
	Top    Side = "top"
	Bottom Side = "bottom"
)

// A closed sheet rests off its edge; open moves it in, and starting: gives
// the first frame of the slide.
var sideClass = gx.Enum[Side]{
	Right:  "inset-y-0 right-0 left-auto h-full w-3/4 translate-x-full border-l open:translate-x-0 starting:open:translate-x-full sm:max-w-sm",
	Left:   "inset-y-0 left-0 right-auto h-full w-3/4 -translate-x-full border-r open:translate-x-0 starting:open:-translate-x-full sm:max-w-sm",
	Top:    "inset-x-0 top-0 bottom-auto h-auto w-full -translate-y-full border-b open:translate-y-0 starting:open:-translate-y-full",
	Bottom: "inset-x-0 bottom-0 top-auto h-auto w-full translate-y-full border-t open:translate-y-0 starting:open:translate-y-full",
}

// sideClass returns the classes of one sheet side; a zero value is Right.
func (p SheetProps) sideClass() string {
	if p.Side == "" {
		return sideClass[Right]
	}
	return sideClass[p.Side]
}
