package inputgroup

import (
	"github.com/alternayte/gx"
	"github.com/alternayte/gx/registry/button"
)

// Align is the position of an addon inside an input group.
type Align string

// The alignments of inputgroup.InputGroupAddon.
const (
	InlineStart Align = "inline-start"
	InlineEnd   Align = "inline-end"
	BlockStart  Align = "block-start"
	BlockEnd    Align = "block-end"
)

var alignClass = gx.Enum[Align]{
	InlineStart: "order-first pl-3 has-[>button]:ml-[-0.45rem] has-[>kbd]:ml-[-0.35rem]",
	InlineEnd:   "order-last pr-3 has-[>button]:mr-[-0.45rem] has-[>kbd]:mr-[-0.35rem]",
	BlockStart:  "order-first w-full justify-start px-3 pt-3 group-has-[>input]/input-group:pt-2.5 [.border-b]:pb-3",
	BlockEnd:    "order-last w-full justify-start px-3 pb-3 group-has-[>input]/input-group:pb-2.5 [.border-t]:pt-3",
}

// align returns the data-align value; a zero value is InlineStart. The
// group reads this attribute to pad its control.
func (p InputGroupAddonProps) align() string {
	if p.Align == "" {
		return string(InlineStart)
	}
	return string(p.Align)
}

// Size is the height and padding of a button inside an input group.
type Size string

// The sizes of inputgroup.InputGroupButton.
const (
	Xs     Size = "xs"
	Sm     Size = "sm"
	IconXs Size = "icon-xs"
	IconSm Size = "icon-sm"
)

var sizeClass = gx.Enum[Size]{
	Xs:     "h-6 gap-1 rounded-[calc(var(--radius)-5px)] px-2 has-[>svg]:px-2 [&>svg:not([class*='size-'])]:size-3.5",
	Sm:     "h-8 gap-1.5 rounded-md px-2.5 has-[>svg]:px-2.5",
	IconXs: "size-6 rounded-[calc(var(--radius)-5px)] p-0 has-[>svg]:p-0",
	IconSm: "size-8 p-0 has-[>svg]:p-0",
}

// buttonSize is the button size that each group size starts from: the
// group classes replace its height and padding.
var buttonSize = map[Size]button.Size{
	Xs:     button.Md,
	Sm:     button.Md,
	IconXs: button.Icon,
	IconSm: button.Icon,
}

// size returns the button size; a zero value is Xs.
func (p InputGroupButtonProps) size() Size {
	if p.Size == "" {
		return Xs
	}
	return p.Size
}

// variant returns the button variant; a zero value is Ghost.
func (p InputGroupButtonProps) variant() button.Variant {
	if p.Variant == "" {
		return button.Ghost
	}
	return p.Variant
}

// inputType returns the type of the input; a zero value is text.
func (p InputGroupInputProps) inputType() string {
	if p.Type == "" {
		return "text"
	}
	return p.Type
}

// attrs returns the id when the input has one, then the caller's
// attributes.
func (p InputGroupInputProps) attrs() gx.Attrs {
	if p.Id == "" {
		return p.Attrs
	}
	return append(gx.Attrs{{Key: "id", Value: p.Id}}, p.Attrs...)
}

// attrs returns the id when the textarea has one, then the caller's
// attributes.
func (p InputGroupTextareaProps) attrs() gx.Attrs {
	if p.Id == "" {
		return p.Attrs
	}
	return append(gx.Attrs{{Key: "id", Value: p.Id}}, p.Attrs...)
}
