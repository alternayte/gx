package avatar

import "github.com/alternayte/gx"

// Size is the diameter of an avatar.
type Size string

// The sizes of avatar.Avatar.
const (
	Sm Size = "sm"
	Md Size = "md"
	Lg Size = "lg"
)

var sizeClass = gx.Enum[Size]{
	Sm: "size-6 text-xs",
	Md: "size-8",
	Lg: "size-10 text-base",
}
