package badge

import "github.com/alternayte/gx"

// Variant is the visual style of a badge.
type Variant string

// The variants of badge.Badge.
const (
	Default     Variant = "default"
	Secondary   Variant = "secondary"
	Destructive Variant = "destructive"
	Outline     Variant = "outline"
)

var variantClass = gx.Enum[Variant]{
	Default:     "border-transparent bg-primary text-primary-foreground",
	Secondary:   "border-transparent bg-secondary text-secondary-foreground",
	Destructive: "border-transparent bg-destructive text-white",
	Outline:     "text-foreground",
}

// class returns the classes of one badge.
func (p BadgeProps) class() string {
	const base = "inline-flex w-fit shrink-0 items-center justify-center gap-1 overflow-hidden rounded-md border px-2 py-0.5 text-xs font-medium whitespace-nowrap transition-[color,box-shadow] focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/50 [&>svg]:pointer-events-none [&>svg]:size-3"
	return gx.Cx(base, variantClass[p.Variant], p.Class)
}
