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

var sideClass = gx.Enum[Side]{
	Right:  "inset-y-0 right-0 left-auto h-full w-3/4 max-w-sm rounded-l-xl border-l",
	Left:   "inset-y-0 left-0 right-auto h-full w-3/4 max-w-sm rounded-r-xl border-r",
	Top:    "inset-x-0 top-0 bottom-auto w-full rounded-b-xl border-b",
	Bottom: "inset-x-0 bottom-0 top-auto w-full rounded-t-xl border-t",
}

// sideClass returns the classes of one sheet side; a zero value is Right.
func (p SheetProps) sideClass() string {
	if p.Side == "" {
		return sideClass[Right]
	}
	return sideClass[p.Side]
}
