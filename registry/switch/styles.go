package switches

import "github.com/alternayte/gx"

// Size is the size of a switch.
type Size string

// The sizes of switches.Switch.
const (
	Md Size = "default"
	Sm Size = "sm"
)

var trackClass = gx.Enum[Size]{
	Md: "h-[1.15rem] w-8",
	Sm: "h-3.5 w-6",
}

var thumbClass = gx.Enum[Size]{
	Md: "size-4",
	Sm: "size-3",
}

// size returns the size of one switch; a zero value is Md.
func (p SwitchProps) size() Size {
	if p.Size == "" {
		return Md
	}
	return p.Size
}
