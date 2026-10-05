package empty

import "github.com/alternayte/gx"

// Variant is the look of the media slot of an empty state.
type Variant string

// The variants of empty.EmptyMedia.
const (
	Default Variant = "default"
	Icon    Variant = "icon"
)

var variantClass = gx.Enum[Variant]{
	Default: "bg-transparent",
	Icon:    "flex size-10 shrink-0 items-center justify-center rounded-lg bg-muted text-foreground [&_svg:not([class*='size-'])]:size-6",
}

// variant returns the data-variant value; a zero value is Default.
func (p EmptyMediaProps) variant() string {
	if p.Variant == "" {
		return string(Default)
	}
	return string(p.Variant)
}
