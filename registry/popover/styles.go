package popover

import (
	"github.com/alternayte/gx"
	"github.com/alternayte/gx/registry/button"
)

// Align is the edge of the trigger that the popover lines up with.
type Align string

// The alignments of popover.Popover.
const (
	Center Align = "center"
	Start  Align = "start"
	End    Align = "end"
)

// The popover zooms from the edge that touches the trigger.
var alignClass = gx.Enum[Align]{
	Center: "origin-top",
	Start:  "origin-top-left",
	End:    "origin-top-right",
}

var alignStyle = gx.Enum[Align]{
	Center: "justify-self: anchor-center",
	Start:  "left: anchor(left)",
	End:    "right: anchor(right)",
}

// align returns the alignment of one popover; a zero value is Center.
func (p PopoverProps) align() Align {
	if p.Align == "" {
		return Center
	}
	return p.Align
}

// style anchors the popover below its trigger. A browser without anchor
// positioning keeps the popover at its place in the document flow.
func (p PopoverProps) style() gx.Style {
	return gx.Style("position-anchor: --gx-pop-" + p.Id + "; inset: auto; margin: 0.25rem 0 0; top: anchor(bottom); " + alignStyle[p.align()])
}

// variant returns the button variant of one trigger; a zero value is
// button.Outline.
func (p PopoverTriggerProps) variant() button.Variant {
	if p.Variant == "" {
		return button.Outline
	}
	return p.Variant
}

// attrs wires the trigger to its popover and names it as the anchor.
func (p PopoverTriggerProps) attrs() gx.Attrs {
	return gx.JoinAttrs(gx.Attrs{
		{Key: "popovertarget", Value: p.Id},
		{Key: "popovertargetaction", Value: "toggle"},
		{Key: "style", Value: "anchor-name: --gx-pop-" + p.Id, Kind: gx.AttrStyle},
	}, p.Attrs)
}

// motionClass fades and zooms the popover from 95% and slides it 2 units from
// the trigger. Safari 26.0 never ends a display transition on an element
// that CSS anchor positioning places, which leaves a closed popover rendered.
// The @supports test matches every engine but WebKit, so Safari closes the
// popover at once and still animates the enter.
const motionClass = "opacity-0 scale-95 transition-[opacity,scale,translate,overlay,display] not-supports-[font:-apple-system-body]:transition-discrete duration-150 open:opacity-100 open:scale-100 starting:open:opacity-0 starting:open:scale-95 starting:open:-translate-y-2 motion-reduce:transition-none"
