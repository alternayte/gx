package button

import "github.com/alternayte/gx"

// Variant is the visual style of a button.
type Variant string

// The variants of button.Button.
const (
	Default     Variant = "default"
	Secondary   Variant = "secondary"
	Destructive Variant = "destructive"
	Outline     Variant = "outline"
	Ghost       Variant = "ghost"
	Link        Variant = "link"
)

var variantClass = gx.Enum[Variant]{
	Default:     "bg-primary text-primary-foreground shadow-xs hover:bg-primary/90",
	Secondary:   "bg-secondary text-secondary-foreground shadow-xs hover:bg-secondary/80",
	Destructive: "bg-destructive text-white shadow-xs hover:bg-destructive/90 focus-visible:ring-destructive/20",
	Outline:     "border bg-background shadow-xs hover:bg-accent hover:text-accent-foreground",
	Ghost:       "hover:bg-accent hover:text-accent-foreground",
	Link:        "text-primary underline-offset-4 hover:underline",
}

// Size is the height and padding of a button.
type Size string

// The sizes of button.Button.
const (
	Sm   Size = "sm"
	Md   Size = "md"
	Lg   Size = "lg"
	Icon Size = "icon"
)

var sizeClass = gx.Enum[Size]{
	Sm:   "h-8 rounded-md gap-1.5 px-3 has-[>svg]:px-2.5",
	Md:   "h-9 rounded-md px-4 py-2 has-[>svg]:px-3",
	Lg:   "h-10 rounded-md px-6 has-[>svg]:px-4",
	Icon: "size-9",
}

// typeAttr returns the button type; a zero value is "button".
func (p ButtonProps) typeAttr() string {
	if p.Type == "" {
		return "button"
	}
	return p.Type
}

// class returns the classes of one button state.
func (p ButtonProps) class() string {
	const base = "inline-flex shrink-0 items-center justify-center gap-2 whitespace-nowrap rounded-md text-sm font-medium outline-none transition-all focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/50 disabled:pointer-events-none disabled:opacity-50 aria-invalid:border-destructive aria-invalid:ring-destructive/20 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4"
	// A zero-value prop is the default, so a Go caller and a .gx caller
	// render the same markup.
	variant := p.Variant
	if variant == "" {
		variant = Default
	}
	size := p.Size
	if size == "" {
		size = Md
	}
	return gx.Cx(base, variantClass[variant], sizeClass[size], p.Class)
}
