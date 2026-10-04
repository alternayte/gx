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
	Default: "",
	Icon:    "flex size-10 items-center justify-center rounded-lg bg-muted text-foreground [&>svg]:size-5",
}

// variantClass returns the classes of one media variant; a zero value is
// Default.
func (p EmptyMediaProps) variantClass() string {
	if p.Variant == "" {
		return variantClass[Default]
	}
	return variantClass[p.Variant]
}
