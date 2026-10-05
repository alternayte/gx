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
	Ghost       Variant = "ghost"
	Link        Variant = "link"
)

var variantClass = gx.Enum[Variant]{
	Default:     "bg-primary text-primary-foreground [a&]:hover:bg-primary/90",
	Secondary:   "bg-secondary text-secondary-foreground [a&]:hover:bg-secondary/90",
	Destructive: "bg-destructive text-white focus-visible:ring-destructive/20 dark:bg-destructive/60 dark:focus-visible:ring-destructive/40 [a&]:hover:bg-destructive/90",
	Outline:     "border-border text-foreground [a&]:hover:bg-accent [a&]:hover:text-accent-foreground",
	Ghost:       "[a&]:hover:bg-accent [a&]:hover:text-accent-foreground",
	Link:        "text-primary underline-offset-4 [a&]:hover:underline",
}

// variant returns the data-variant value; a zero value is Default.
func (p BadgeProps) variant() string {
	if p.Variant == "" {
		return string(Default)
	}
	return string(p.Variant)
}

// class returns the classes of one badge.
func (p BadgeProps) class() string {
	const base = "inline-flex w-fit shrink-0 items-center justify-center gap-1 overflow-hidden rounded-full border border-transparent px-2 py-0.5 text-xs font-medium whitespace-nowrap transition-[color,box-shadow] focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 aria-invalid:border-destructive aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40 [&>svg]:pointer-events-none [&>svg]:size-3"
	return gx.Cx(base, variantClass[Variant(p.variant())], p.Class)
}
