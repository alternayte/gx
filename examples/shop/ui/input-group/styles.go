package inputgroup

import "github.com/alternayte/gx"

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
	BlockStart:  "order-first w-full justify-start px-3 pt-3",
	BlockEnd:    "order-last w-full justify-start px-3 pb-3",
}
