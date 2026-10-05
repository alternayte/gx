package selectbox

import "github.com/alternayte/gx"

// Size is the height of a select.
type Size string

// The sizes of selectbox.Select.
const (
	Md Size = "default"
	Sm Size = "sm"
)

var sizeClass = gx.Enum[Size]{
	Md: "h-9",
	Sm: "h-8",
}

// size returns the size of one select; a zero value is Md.
func (p SelectProps) size() Size {
	if p.Size == "" {
		return Md
	}
	return p.Size
}
