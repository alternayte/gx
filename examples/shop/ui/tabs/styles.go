package tabs

import "github.com/alternayte/gx"

// Orientation is the layout direction of a tab group.
type Orientation string

// The orientations of tabs.Tabs.
const (
	Horizontal Orientation = "horizontal"
	Vertical   Orientation = "vertical"
)

// orientation returns the data-orientation value; a zero value is
// Horizontal. The list, the triggers and the behaviour runtime read it.
func (p TabsProps) orientation() string {
	if p.Orientation == "" {
		return string(Horizontal)
	}
	return string(p.Orientation)
}

// Variant is the visual style of a tab list.
type Variant string

// The variants of tabs.TabsList.
const (
	Default Variant = "default"
	Line    Variant = "line"
)

var variantClass = gx.Enum[Variant]{
	Default: "bg-muted",
	Line:    "gap-1 rounded-none bg-transparent",
}

// variant returns the variant of one list; a zero value is Default.
func (p TabsListProps) variant() Variant {
	if p.Variant == "" {
		return Default
	}
	return p.Variant
}
