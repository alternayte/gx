package textarea

import (
	"strconv"

	"github.com/alternayte/gx"
)

// Control is the classes of a textarea without its focus ring. A group
// that draws the ring on its own border takes these classes for its
// control.
const Control = "flex field-sizing-content min-h-16 w-full rounded-md border border-input bg-transparent px-3 py-2 text-base shadow-xs transition-[color,box-shadow] outline-none placeholder:text-muted-foreground disabled:cursor-not-allowed disabled:opacity-50 aria-invalid:border-destructive aria-invalid:ring-destructive/20 md:text-sm dark:bg-input/30 dark:aria-invalid:ring-destructive/40"

// Class is the classes of a textarea. A control that looks like a textarea
// takes these classes.
const Class = Control + " focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"

// attrs returns the rows attribute when the caller sets one, then the
// caller's attributes. The textarea grows with its content, so rows only
// counts in a browser without field-sizing.
func (p TextareaProps) attrs() gx.Attrs {
	if p.Rows <= 0 {
		return p.Attrs
	}
	return append(gx.Attrs{{Key: "rows", Value: strconv.Itoa(p.Rows)}}, p.Attrs...)
}
