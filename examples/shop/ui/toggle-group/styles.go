package togglegroup

import (
	"strconv"

	"github.com/alternayte/gx"
)

// Variant is the visual style of the items of a toggle group.
type Variant string

// The variants of togglegroup.ToggleGroup.
const (
	Default Variant = "default"
	Outline Variant = "outline"
)

// Size is the height of the items of a toggle group.
type Size string

// The sizes of togglegroup.ToggleGroup.
const (
	Sm Size = "sm"
	Md Size = "md"
	Lg Size = "lg"
)

// variant returns the data-variant value; a zero value is Default. The
// items read it through the group, so one prop styles every item.
func (p ToggleGroupProps) variant() string {
	if p.Variant == "" {
		return string(Default)
	}
	return string(p.Variant)
}

// size returns the data-size value; a zero value is Md.
func (p ToggleGroupProps) size() string {
	if p.Size == "" {
		return string(Md)
	}
	return string(p.Size)
}

// spacing returns the gap between the items in spacing units. Zero joins
// the items.
func (p ToggleGroupProps) spacing() int {
	if p.Spacing < 0 {
		return 0
	}
	return p.Spacing
}

// gap returns the custom property that the gap class of the group reads.
func (p ToggleGroupProps) gap() gx.Style {
	return gx.Style("--gap: " + strconv.Itoa(p.spacing()))
}

// inputType returns the type of the native input: a radio for an exclusive
// choice, a checkbox when the group takes many values.
func (p ToggleGroupItemProps) inputType() string {
	if p.Multiple {
		return "checkbox"
	}
	return "radio"
}
