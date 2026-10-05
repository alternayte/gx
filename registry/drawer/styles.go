package drawer

import "github.com/alternayte/gx"

// Side is the edge a drawer slides from.
type Side string

// The sides of drawer.Drawer.
const (
	Bottom Side = "bottom"
	Top    Side = "top"
	Right  Side = "right"
	Left   Side = "left"
)

// A closed drawer rests off its edge; open moves it in, and starting: gives
// the first frame of the slide.
var sideClass = gx.Enum[Side]{
	Bottom: "inset-x-0 bottom-0 top-auto mt-24 h-auto max-h-[80vh] w-full translate-y-full rounded-t-lg border-t open:translate-y-0 starting:open:translate-y-full",
	Top:    "inset-x-0 top-0 bottom-auto mb-24 h-auto max-h-[80vh] w-full -translate-y-full rounded-b-lg border-b open:translate-y-0 starting:open:-translate-y-full",
	Right:  "inset-y-0 right-0 left-auto h-full w-3/4 translate-x-full border-l open:translate-x-0 starting:open:translate-x-full sm:max-w-sm",
	Left:   "inset-y-0 left-0 right-auto h-full w-3/4 -translate-x-full border-r open:translate-x-0 starting:open:-translate-x-full sm:max-w-sm",
}

// side returns the data-side value; a zero value is Bottom.
func (p DrawerProps) side() string {
	if p.Side == "" {
		return string(Bottom)
	}
	return string(p.Side)
}

// sideClass returns the classes of one drawer side.
func (p DrawerProps) sideClass() string {
	return sideClass[Side(p.side())]
}
