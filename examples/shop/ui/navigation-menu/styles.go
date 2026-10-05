package navigationmenu

import "github.com/alternayte/gx"

// Variant is the visual style of a navigation link.
type Variant string

// The variants of navigationmenu.NavigationMenuLink. Default is a link
// inside the content of a menu. Trigger is a link in the bar; it takes the
// style of a trigger.
const (
	Default Variant = "default"
	Trigger Variant = "trigger"
)

// triggerClass is the style of a trigger and of a link in the bar.
const triggerClass = "inline-flex h-9 w-max items-center justify-center rounded-md bg-background px-4 py-2 text-sm font-medium no-underline transition-[color,box-shadow] outline-none hover:bg-accent hover:text-accent-foreground focus:bg-accent focus:text-accent-foreground focus-visible:ring-[3px] focus-visible:ring-ring/50 focus-visible:outline-1 disabled:pointer-events-none disabled:opacity-50"

var variantClass = gx.Enum[Variant]{
	Default: "",
	Trigger: triggerClass,
}

var activeClass = map[bool]string{
	true:  "bg-accent/50 text-accent-foreground hover:bg-accent focus:bg-accent",
	false: "",
}

// variant returns the variant of one link; a zero value is Default.
func (p NavigationMenuLinkProps) variant() Variant {
	if p.Variant == "" {
		return Default
	}
	return p.Variant
}

// current returns the aria-current value of one link.
func (p NavigationMenuLinkProps) current() string {
	if p.Active {
		return "page"
	}
	return ""
}
