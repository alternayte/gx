package dropdownmenu

import (
	"github.com/alternayte/gx"
	"github.com/alternayte/gx/examples/shop/ui/button"
)

// Align is the edge of the trigger that the menu lines up with.
type Align string

// The alignments of dropdownmenu.DropdownMenu.
const (
	Center Align = "center"
	Start  Align = "start"
	End    Align = "end"
)

// The menu zooms from the edge that touches the trigger. The overlay module
// writes data-side: a menu that flips opens above the trigger.
var alignClass = gx.Enum[Align]{
	Center: "origin-top data-[side=top]:origin-bottom",
	Start:  "origin-top-left data-[side=top]:origin-bottom-left",
	End:    "origin-top-right data-[side=top]:origin-bottom-right",
}

var alignStyle = gx.Enum[Align]{
	Center: "justify-self: anchor-center",
	Start:  "left: anchor(left)",
	End:    "right: anchor(right)",
}

// align returns the alignment of one menu; a zero value is Center.
func (p DropdownMenuProps) align() Align {
	if p.Align == "" {
		return Center
	}
	return p.Align
}

// style anchors the menu below its trigger for a page whose scripts did not
// run. The overlay module then writes the measured place.
func (p DropdownMenuProps) style() gx.Style {
	return gx.Style("position-anchor: --gx-menu-" + p.Id + "; inset: auto; margin: 0.25rem 0 0; top: anchor(bottom); " + alignStyle[p.align()])
}

// place tells the overlay module where the menu goes: below the trigger,
// 4px away, and inside the viewport.
func (p DropdownMenuProps) place() string {
	return "bottom " + string(p.align()) + " 4"
}

// variant returns the button variant of one trigger; a zero value is
// button.Outline.
func (p DropdownMenuTriggerProps) variant() button.Variant {
	if p.Variant == "" {
		return button.Outline
	}
	return p.Variant
}

// attrs wires the trigger to its menu and names it as the anchor.
func (p DropdownMenuTriggerProps) attrs() gx.Attrs {
	return gx.JoinAttrs(gx.Attrs{
		{Key: "popovertarget", Value: p.Id},
		{Key: "aria-haspopup", Value: "menu"},
		{Key: "style", Value: "anchor-name: --gx-menu-" + p.Id, Kind: gx.AttrStyle},
	}, p.Attrs)
}

// Variant is the visual style of a menu item.
type Variant string

// The variants of dropdownmenu.DropdownMenuItem.
const (
	Default     Variant = "default"
	Destructive Variant = "destructive"
)

var variantClass = gx.Enum[Variant]{
	Default:     "",
	Destructive: "text-destructive hover:bg-destructive/10 hover:text-destructive focus:bg-destructive/10 focus:text-destructive dark:hover:bg-destructive/20 dark:focus:bg-destructive/20 *:[svg]:text-destructive!",
}

// variant returns the variant of one item; a zero value is Default.
func (p DropdownMenuItemProps) variant() Variant {
	if p.Variant == "" {
		return Default
	}
	return p.Variant
}

// insetClass lines an item or a label up with the checkbox and radio items.
var insetClass = map[bool]string{
	true:  "pl-8",
	false: "",
}

// rovingItem marks an item for the roving tabindex of its menu. A disabled
// item is not marked, so the arrow keys pass it.
func rovingItem(disabled bool) gx.Attrs {
	return gx.Attrs{gx.Bool("data-gx-roving-item", !disabled)}
}

// motionClass fades and zooms the menu from 95% and slides it 2 units from
// the trigger, from below when the menu took the top side. Safari 26.0 never ends a display transition on an element
// that CSS anchor positioning places, which leaves a closed menu rendered.
// The @supports test matches every engine but WebKit, so Safari closes the
// menu at once and still animates the enter.
const motionClass = "opacity-0 scale-95 transition-[opacity,scale,translate,overlay,display] not-supports-[font:-apple-system-body]:transition-discrete duration-150 open:opacity-100 open:scale-100 starting:open:opacity-0 starting:open:scale-95 starting:open:-translate-y-2 data-[side=top]:starting:open:translate-y-2 motion-reduce:transition-none"

// subMotionClass fades and zooms the content of a sub-menu from 95% and
// slides it 2 units from its trigger. The overlay module writes data-side:
// content that flips to the left slides from the right.
const subMotionClass = "origin-top-left opacity-0 scale-95 transition-[opacity,scale,translate,overlay,display] transition-discrete duration-150 open:opacity-100 open:scale-100 starting:open:opacity-0 starting:open:scale-95 starting:open:-translate-x-2 data-[side=left]:origin-top-right data-[side=left]:starting:open:translate-x-2 motion-reduce:transition-none"
