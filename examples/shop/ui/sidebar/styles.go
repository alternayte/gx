package sidebar

import "github.com/alternayte/gx"

// Side is the edge of the page that holds the sidebar.
type Side string

// The sides of sidebar.Sidebar.
const (
	Left  Side = "left"
	Right Side = "right"
)

// side returns the data-side value; a zero value is Left. The border of
// the sidebar follows this attribute.
func (p SidebarProps) side() string {
	if p.Side == "" {
		return string(Left)
	}
	return string(p.Side)
}

// Variant is the surface of a menu button.
type Variant string

// The variants of sidebar.SidebarMenuButton.
const (
	Default Variant = "default"
	Outline Variant = "outline"
)

var variantClass = gx.Enum[Variant]{
	Default: "hover:bg-sidebar-accent hover:text-sidebar-accent-foreground",
	Outline: "bg-background shadow-[0_0_0_1px_var(--sidebar-border)] hover:bg-sidebar-accent hover:text-sidebar-accent-foreground hover:shadow-[0_0_0_1px_var(--sidebar-accent)]",
}

// Size is the height and the text size of a menu button.
type Size string

// The sizes of sidebar.SidebarMenuButton. A sub button has Md and Sm.
const (
	Md Size = "default"
	Sm Size = "sm"
	Lg Size = "lg"
)

var sizeClass = gx.Enum[Size]{
	Md: "h-8 text-sm",
	Sm: "h-7 text-xs",
	Lg: "h-12 text-sm",
}

// subSizeClass is the text size of a sub button. Its height is fixed.
var subSizeClass = gx.Enum[Size]{
	Md: "text-sm",
	Sm: "text-xs",
	Lg: "text-sm",
}

// hoverClass hides a menu action on a wide screen until its item has the
// pointer or the focus.
var hoverClass = map[bool]string{
	true:  "group-focus-within/menu-item:opacity-100 group-hover/menu-item:opacity-100 peer-aria-[current=page]/menu-button:text-sidebar-accent-foreground peer-data-active/menu-button:text-sidebar-accent-foreground data-[state=open]:opacity-100 md:opacity-0",
	false: "",
}

// size returns the data-size value; a zero value is Md. The action and the
// badge of the item read this attribute.
func (p SidebarMenuButtonProps) size() string {
	if p.Size == "" {
		return string(Md)
	}
	return string(p.Size)
}

// class returns the classes of one menu button.
func (p SidebarMenuButtonProps) class() string {
	const base = "peer/menu-button flex w-full items-center gap-2 overflow-hidden rounded-md p-2 text-left text-sm ring-sidebar-ring outline-hidden transition-[width,height,padding] motion-reduce:transition-none group-has-data-[sidebar=menu-action]/menu-item:pr-8 hover:bg-sidebar-accent hover:text-sidebar-accent-foreground focus-visible:ring-2 active:bg-sidebar-accent active:text-sidebar-accent-foreground disabled:pointer-events-none disabled:opacity-50 aria-disabled:pointer-events-none aria-disabled:opacity-50 aria-[current=page]:bg-sidebar-accent aria-[current=page]:font-medium aria-[current=page]:text-sidebar-accent-foreground data-active:bg-sidebar-accent data-active:font-medium data-active:text-sidebar-accent-foreground data-[state=open]:hover:bg-sidebar-accent data-[state=open]:hover:text-sidebar-accent-foreground [&>span:last-child]:truncate [&>svg]:size-4 [&>svg]:shrink-0"
	variant := p.Variant
	if variant == "" {
		variant = Default
	}
	return gx.Cx(base, variantClass[variant], sizeClass[Size(p.size())], activeClass[p.Active], p.Class)
}

// activeClass is the look of a menu button with Active set. A link also
// takes this look from aria-current and data-active, which the framework
// keeps current for a typed href.
var activeClass = map[bool]string{
	true:  "bg-sidebar-accent font-medium text-sidebar-accent-foreground",
	false: "",
}

// subActiveClass is the look of a sub button with Active set.
var subActiveClass = map[bool]string{
	true:  "bg-sidebar-accent text-sidebar-accent-foreground",
	false: "",
}

// attrs marks the link of the current page, then adds the caller's
// attributes.
func (p SidebarMenuButtonProps) attrs() gx.Attrs {
	if !p.Active {
		return p.Attrs
	}
	return append(gx.Attrs{{Key: "aria-current", Value: "page"}}, p.Attrs...)
}

// buttonAttrs marks an active button, then adds the caller's attributes.
// The action and the badge of the item read data-active.
func (p SidebarMenuButtonProps) buttonAttrs() gx.Attrs {
	if !p.Active {
		return p.Attrs
	}
	return append(gx.Attrs{{Key: "data-active", Value: "true"}}, p.Attrs...)
}

// size returns the data-size value; a zero value is Md.
func (p SidebarMenuSubButtonProps) size() string {
	if p.Size == "" {
		return string(Md)
	}
	return string(p.Size)
}

// attrs marks the link of the current page, then adds the caller's
// attributes.
func (p SidebarMenuSubButtonProps) attrs() gx.Attrs {
	if !p.Active {
		return p.Attrs
	}
	return append(gx.Attrs{{Key: "aria-current", Value: "page"}}, p.Attrs...)
}

// width returns the width of the text bar; a zero value is 70%.
func (p SidebarMenuSkeletonProps) width() string {
	if p.Width == "" {
		return "70%"
	}
	return p.Width
}

// inputType returns the type of the input; a zero value is text.
func (p SidebarInputProps) inputType() string {
	if p.Type == "" {
		return "text"
	}
	return p.Type
}

// controls returns the id of the sidebar; a zero value is gx-sidebar, the
// id the shell runtime shows and hides.
func (p SidebarTriggerProps) controls() string {
	if p.Controls == "" {
		return "gx-sidebar"
	}
	return p.Controls
}

// attrs returns the attributes of the trigger: the shell runtime reads
// data-gx-menu and keeps aria-expanded current.
func (p SidebarTriggerProps) attrs() gx.Attrs {
	return append(gx.Attrs{
		{Key: "data-sidebar", Value: "trigger"},
		gx.Bool("data-gx-menu", true),
		{Key: "aria-controls", Value: p.controls()},
		{Key: "aria-expanded", Value: "false"},
	}, p.Attrs...)
}
