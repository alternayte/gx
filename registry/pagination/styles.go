package pagination

import "github.com/alternayte/gx"

// Size is the shape of a pagination link.
type Size string

// The sizes of pagination.PaginationLink.
const (
	Icon Size = "icon"
	Text Size = "text"
)

var sizeClass = gx.Enum[Size]{
	Icon: "size-9",
	Text: "h-9 px-3",
}

// sizeClass returns the classes of one link size; a zero value is Icon.
func (p PaginationLinkProps) sizeClass() string {
	if p.Size == "" {
		return sizeClass[Icon]
	}
	return sizeClass[p.Size]
}

// current returns the aria-current value of one link.
func (p PaginationLinkProps) current() string {
	if p.Active {
		return "page"
	}
	return ""
}

// class returns the classes of one pagination link.
func (p PaginationLinkProps) class() string {
	base := "inline-flex items-center justify-center gap-1 rounded-md text-sm font-medium whitespace-nowrap transition-colors hover:bg-accent hover:text-accent-foreground focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:pointer-events-none disabled:opacity-50"
	if p.Active {
		return gx.Cx(base, p.sizeClass(), "border border-border", p.Class)
	}
	return gx.Cx(base, p.sizeClass(), p.Class)
}

// attrs returns the link attributes of the previous link.
func (p PaginationPreviousProps) attrs() gx.Attrs {
	return append(gx.Attrs{{Key: "aria-label", Value: "Go to previous page"}}, p.Attrs...)
}

// attrs returns the link attributes of the next link.
func (p PaginationNextProps) attrs() gx.Attrs {
	return append(gx.Attrs{{Key: "aria-label", Value: "Go to next page"}}, p.Attrs...)
}
