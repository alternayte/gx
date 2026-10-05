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

// The menu zooms from the edge that touches the trigger.
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

// align returns the alignment of one menu; a zero value is Center.
func (p DropdownMenuProps) align() Align {
	if p.Align == "" {
		return Center
	}
	return p.Align
}

// style anchors the menu below its trigger. A browser without anchor
// positioning keeps the menu at its place in the document flow.
func (p DropdownMenuProps) style() gx.Style {
	return gx.Style("position-anchor: --gx-menu-" + p.Id + "; inset: auto; margin: 0.25rem 0 0; top: anchor(bottom); " + alignStyle[p.align()])
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
// the trigger. Safari 26.0 never ends a display transition on an element
// that CSS anchor positioning places, which leaves a closed menu rendered.
// The @supports test matches every engine but WebKit, so Safari closes the
// menu at once and still animates the enter.
const motionClass = "opacity-0 scale-95 transition-[opacity,scale,translate,overlay,display] not-supports-[font:-apple-system-body]:transition-discrete duration-150 open:opacity-100 open:scale-100 starting:open:opacity-0 starting:open:scale-95 starting:open:-translate-y-2 motion-reduce:transition-none"
