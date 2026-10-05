package hovercard

import "github.com/alternayte/gx"

// Align is the edge of the trigger that the card lines up with.
type Align string

// The alignments of hovercard.HoverCard. The card has no collision handling,
// so the default keeps it inside the page next to a trigger at the left
// edge.
const (
	Start  Align = "start"
	Center Align = "center"
	End    Align = "end"
)

var alignClass = gx.Enum[Align]{
	Start:  "left-0 origin-top-left",
	Center: "left-[round(50%,1px)] origin-top -translate-x-[round(50%,1px)]",
	End:    "right-0 origin-top-right",
}

// align returns the alignment of one card; a zero value is Start.
func (p HoverCardProps) align() Align {
	if p.Align == "" {
		return Start
	}
	return p.Align
}
