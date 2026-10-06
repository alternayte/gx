// Package resizable is a group of panels with a handle between two panels.
// The ResizableDrag island moves a handle with the pointer and the keyboard.
package resizable

import (
	"strconv"

	"github.com/alternayte/gx"
)

// Orientation is the direction of a panel group.
type Orientation string

// The orientations of resizable.ResizablePanelGroup.
const (
	Horizontal Orientation = "horizontal"
	Vertical   Orientation = "vertical"
)

// ResizableDragProps are the props of the island of one handle. The island
// changes the sizes of the panel before and the panel after the element
// with the id Handle.
type ResizableDragProps struct {
	// Handle is the id of the handle element.
	Handle string `json:"handle"`
	// Step is the size of one arrow key step, in percent of the group.
	Step int `json:"step"`
	// GripClass is the classes of the grip in the middle of the handle. It
	// is a Go constant, so the stylesheet build sees it.
	GripClass string `json:"gripClass"`
}

var groupClass = gx.Enum[Orientation]{
	Horizontal: "flex h-full w-full flex-row",
	Vertical:   "flex h-full w-full flex-col",
}

// panelClass is the classes of one panel. A panel with no size shares the
// free space.
const panelClass = "min-h-0 min-w-0 flex-1 overflow-auto"

// contentClass is the classes of the content element of a panel. The
// classes of the user go here: padding on the panel itself would change its
// part of the group.
const contentClass = "h-full w-full"

// handleClass is the classes of a handle: a line with a wider area for the
// pointer.
var handleClass = gx.Enum[Orientation]{
	Horizontal: "relative flex w-px shrink-0 cursor-col-resize touch-none items-center justify-center bg-border outline-none after:absolute after:inset-y-0 after:left-1/2 after:w-2 after:-translate-x-1/2 focus-visible:ring-[3px] focus-visible:ring-ring/50 data-[dragging=true]:bg-ring",
	Vertical:   "relative flex h-px w-full shrink-0 cursor-row-resize touch-none items-center justify-center bg-border outline-none after:absolute after:inset-x-0 after:top-1/2 after:h-2 after:-translate-y-1/2 focus-visible:ring-[3px] focus-visible:ring-ring/50 data-[dragging=true]:bg-ring",
}

// gripClass is the classes of the grip that the island shows on a handle.
var gripClass = gx.Enum[Orientation]{
	Horizontal: "z-10 block h-4 w-1.5 rounded-xs border border-border bg-border",
	Vertical:   "z-10 block h-1.5 w-4 rounded-xs border border-border bg-border",
}

// orientation returns the direction; a zero value is Horizontal.
func (p ResizablePanelGroupProps) orientation() Orientation {
	if p.Orientation == Vertical {
		return Vertical
	}
	return Horizontal
}

// orientation returns the direction of the group; a zero value is
// Horizontal.
func (p ResizableHandleProps) orientation() Orientation {
	if p.Orientation == Vertical {
		return Vertical
	}
	return Horizontal
}

// ariaOrientation returns the direction of the separator line, which is
// across the direction of the group.
func (p ResizableHandleProps) ariaOrientation() string {
	if p.orientation() == Vertical {
		return "horizontal"
	}
	return "vertical"
}

// label returns the accessible name; a zero value is "Resize panels".
func (p ResizableHandleProps) label() string {
	if p.Label == "" {
		return "Resize panels"
	}
	return p.Label
}

// step returns the arrow key step; a zero value is 5.
func (p ResizableHandleProps) step() int {
	if p.Step <= 0 {
		return 5
	}
	return p.Step
}

// maxSize returns the largest size; a zero value is 100.
func (p ResizablePanelProps) maxSize() int {
	if p.MaxSize <= 0 {
		return 100
	}
	return p.MaxSize
}

// style gives a panel with a size its part of the group.
func (p ResizablePanelProps) style() gx.Style {
	if p.Size <= 0 {
		return ""
	}
	// The size is the weight of the panel in the room that the panels
	// share. The handles take none of that room.
	return gx.Style("flex: " + strconv.Itoa(p.Size) + " 1 0%")
}
