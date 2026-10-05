package toggle

import "github.com/alternayte/gx"

// Variant is the visual style of a toggle.
type Variant string

// The variants of toggle.Toggle.
const (
	Default Variant = "default"
	Outline Variant = "outline"
)

var variantClass = gx.Enum[Variant]{
	Default: "bg-transparent",
	Outline: "border border-input bg-transparent shadow-xs hover:bg-accent hover:text-accent-foreground",
}

// Size is the height and padding of a toggle.
type Size string

// The sizes of toggle.Toggle.
const (
	Sm Size = "sm"
	Md Size = "md"
	Lg Size = "lg"
)

var sizeClass = gx.Enum[Size]{
	Sm: "h-8 min-w-8 px-1.5",
	Md: "h-9 min-w-9 px-2",
	Lg: "h-10 min-w-10 px-2.5",
}

// class returns the classes of one toggle. The label carries the recipe and
// reads the state of the checkbox inside it.
func (p ToggleProps) class() string {
	const base = "inline-flex items-center justify-center gap-2 rounded-md text-sm font-medium whitespace-nowrap transition-[color,box-shadow] outline-none hover:bg-muted hover:text-muted-foreground has-[:focus-visible]:border-ring has-[:focus-visible]:ring-[3px] has-[:focus-visible]:ring-ring/50 has-[:disabled]:pointer-events-none has-[:disabled]:opacity-50 has-[[aria-invalid=true]]:border-destructive has-[[aria-invalid=true]]:ring-destructive/20 has-[:checked]:bg-accent has-[:checked]:text-accent-foreground dark:has-[[aria-invalid=true]]:ring-destructive/40 motion-reduce:transition-none [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4"
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

// invalid returns the aria-invalid value of the input.
func (p ToggleProps) invalid() string {
	if p.Invalid {
		return "true"
	}
	return "false"
}
