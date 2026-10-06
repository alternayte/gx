// Package carousel is a row of slides in a scroll container with CSS scroll
// snap. The CarouselControls island adds the previous and next buttons.
package carousel

import "github.com/alternayte/gx"

// Orientation is the direction of the slides of a carousel.
type Orientation string

// The orientations of carousel.Carousel.
const (
	Horizontal Orientation = "horizontal"
	Vertical   Orientation = "vertical"
)

// CarouselControlsProps are the props of the island that draws the
// buttons. The island scrolls the element with the id Viewport.
type CarouselControlsProps struct {
	// Viewport is the id of the scroll container.
	Viewport string `json:"viewport"`
	// Orientation is "horizontal" or "vertical".
	Orientation string `json:"orientation"`
	// PreviousLabel and NextLabel name the two buttons.
	PreviousLabel string `json:"previousLabel"`
	NextLabel     string `json:"nextLabel"`
	// StatusText is the text of the position, with {n} and {count}.
	StatusText string `json:"statusText"`
	// Classes holds the classes of the parts. They are Go constants, so
	// the stylesheet build sees them.
	Classes ControlClasses `json:"classes"`
}

// ControlClasses are the classes of the parts that the island draws.
type ControlClasses struct {
	Root   string `json:"root"`
	Button string `json:"button"`
	Status string `json:"status"`
}

const rootClass = "flex w-full max-w-sm flex-col gap-2"

// viewportClass is the classes of the scroll container. The slides snap to
// its start.
var viewportClass = gx.Enum[Orientation]{
	Horizontal: "flex snap-x snap-mandatory overflow-x-auto overflow-y-hidden rounded-md outline-none [scrollbar-width:none] focus-visible:ring-[3px] focus-visible:ring-ring/50",
	Vertical:   "flex h-64 snap-y snap-mandatory flex-col overflow-x-hidden overflow-y-auto rounded-md outline-none [scrollbar-width:none] focus-visible:ring-[3px] focus-visible:ring-ring/50",
}

// itemClass is the classes of one slide: as wide and as high as the scroll
// container.
const itemClass = "min-h-0 min-w-0 shrink-0 grow-0 basis-full snap-start"

// statusText is the text of the position. The island fills in the numbers.
const statusText = "Slide {n} of {count}"

var controlClasses = ControlClasses{
	Root:   "flex items-center justify-center gap-3",
	Button: "inline-flex size-8 items-center justify-center rounded-full border border-border bg-background text-sm shadow-xs outline-none hover:bg-accent hover:text-accent-foreground focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:pointer-events-none disabled:opacity-50",
	Status: "min-w-24 text-center text-sm text-muted-foreground",
}

// label returns the accessible name; a zero value is "Carousel".
func (p CarouselProps) label() string {
	if p.Label == "" {
		return "Carousel"
	}
	return p.Label
}

// orientation returns the direction; a zero value is Horizontal.
func (p CarouselProps) orientation() Orientation {
	if p.Orientation == Vertical {
		return Vertical
	}
	return Horizontal
}

func (p CarouselProps) viewportID() string { return p.Id + "-viewport" }
