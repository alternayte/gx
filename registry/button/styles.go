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
	Default:     "bg-primary text-primary-foreground hover:bg-primary/90",
	Secondary:   "bg-secondary text-secondary-foreground hover:bg-secondary/80",
	Destructive: "bg-destructive text-white hover:bg-destructive/90 focus-visible:ring-destructive/20 dark:bg-destructive/60 dark:focus-visible:ring-destructive/40",
	Outline:     "border border-border bg-background shadow-xs hover:bg-accent hover:text-accent-foreground dark:border-input dark:bg-input/30 dark:hover:bg-input/50",
	Ghost:       "hover:bg-accent hover:text-accent-foreground dark:hover:bg-accent/50",
	Link:        "text-primary underline-offset-4 hover:underline",
}

// Size is the height and padding of a button.
type Size string

// The sizes of button.Button.
const (
	Xs     Size = "xs"
	Sm     Size = "sm"
	Md     Size = "default"
	Lg     Size = "lg"
	Icon   Size = "icon"
	IconXs Size = "icon-xs"
	IconSm Size = "icon-sm"
	IconLg Size = "icon-lg"
)

var sizeClass = gx.Enum[Size]{
	Xs:     "h-6 gap-1 rounded-md px-2 text-xs has-[>svg]:px-1.5 [&_svg:not([class*='size-'])]:size-3",
	Sm:     "h-8 gap-1.5 rounded-md px-3 has-[>svg]:px-2.5",
	Md:     "h-9 px-4 py-2 has-[>svg]:px-3",
	Lg:     "h-10 rounded-md px-6 has-[>svg]:px-4",
	Icon:   "size-9",
	IconXs: "size-6 rounded-md [&_svg:not([class*='size-'])]:size-3",
	IconSm: "size-8",
	IconLg: "size-10",
}

// typeAttr returns the button type; a zero value is "button".
func (p ButtonProps) typeAttr() string {
	if p.Type == "" {
		return "button"
	}
	return p.Type
}

// variant returns the data-variant value; a zero value is Default.
func (p ButtonProps) variant() string {
	if p.Variant == "" {
		return string(Default)
	}
	return string(p.Variant)
}

// size returns the data-size value; a zero value is Md.
func (p ButtonProps) size() string {
	if p.Size == "" {
		return string(Md)
	}
	return string(p.Size)
}

// Class returns the classes of one button state. A link that looks like a
// button takes these classes on its anchor.
func Class(variant Variant, size Size, extra ...string) string {
	const base = "inline-flex shrink-0 items-center justify-center gap-2 rounded-md text-sm font-medium whitespace-nowrap transition-all outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:pointer-events-none disabled:opacity-50 aria-invalid:border-destructive aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4"
	// A zero-value prop is the default, so a Go caller and a .gx caller
	// render the same markup.
	if variant == "" {
		variant = Default
	}
	if size == "" {
		size = Md
	}
	return gx.Cx(append([]string{base, variantClass[variant], sizeClass[size]}, extra...)...)
}
