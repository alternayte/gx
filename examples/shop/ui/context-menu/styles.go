package contextmenu

import "github.com/alternayte/gx"

// Variant is the visual style of a menu item.
type Variant string

// The variants of contextmenu.ContextMenuItem.
const (
	Default     Variant = "default"
	Destructive Variant = "destructive"
)

var variantClass = gx.Enum[Variant]{
	Default:     "",
	Destructive: "text-destructive hover:bg-destructive/10 hover:text-destructive focus:bg-destructive/10 focus:text-destructive dark:hover:bg-destructive/20 dark:focus:bg-destructive/20 *:[svg]:text-destructive!",
}

// variant returns the variant of one item; a zero value is Default.
func (p ContextMenuItemProps) variant() Variant {
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

// subMotionClass fades and zooms the content of a sub-menu from 95% and
// slides it 2 units from its trigger. The overlay module writes data-side:
// content that flips to the left slides from the right.
const subMotionClass = "origin-top-left opacity-0 scale-95 transition-[opacity,scale,translate,overlay,display] transition-discrete duration-150 open:opacity-100 open:scale-100 starting:open:opacity-0 starting:open:scale-95 starting:open:-translate-x-2 data-[side=left]:origin-top-right data-[side=left]:starting:open:translate-x-2 motion-reduce:transition-none"
