package menubar

import "github.com/alternayte/gx"

// triggerStyle names the trigger of one menu as the anchor of its content.
func (p MenubarMenuProps) triggerStyle() gx.Style {
	return gx.Style("anchor-name: --gx-menubar-" + p.Id)
}

// contentStyle anchors the content below its trigger, 8px down and 4px to
// the left as the reference offsets do, for a page whose scripts did not
// run. The overlay module then writes the measured place from the same
// offsets.
func (p MenubarMenuProps) contentStyle() gx.Style {
	return gx.Style("position-anchor: --gx-menubar-" + p.Id + "; inset: auto; margin: 0.5rem 0 0 -0.25rem; top: anchor(bottom); left: anchor(left)")
}

// Variant is the visual style of a menu item.
type Variant string

// The variants of menubar.MenubarItem.
const (
	Default     Variant = "default"
	Destructive Variant = "destructive"
)

var variantClass = gx.Enum[Variant]{
	Default:     "",
	Destructive: "text-destructive hover:bg-destructive/10 hover:text-destructive focus:bg-destructive/10 focus:text-destructive dark:hover:bg-destructive/20 dark:focus:bg-destructive/20 *:[svg]:text-destructive!",
}

// variant returns the variant of one item; a zero value is Default.
func (p MenubarItemProps) variant() Variant {
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
