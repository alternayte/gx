package spinner

import "github.com/alternayte/gx"

// Size is the diameter of a spinner.
type Size string

// The sizes of spinner.Spinner.
const (
	Sm Size = "sm"
	Md Size = "md"
	Lg Size = "lg"
)

var sizeClass = gx.Enum[Size]{
	Sm: "size-4",
	Md: "size-5",
	Lg: "size-8",
}
