package pagination

import (
	"github.com/alternayte/gx"
	"github.com/alternayte/gx/examples/shop/ui/button"
)

// class returns the classes of one pagination link: the outline button for
// the current page, the ghost button for the others. A zero size is Icon.
func (p PaginationLinkProps) class() string {
	size := p.Size
	if size == "" {
		size = button.Icon
	}
	if p.Active {
		return button.Class(button.Outline, size, p.Class)
	}
	return button.Class(button.Ghost, size, p.Class)
}

// attrs returns the state attributes of one link, then the caller's.
func (p PaginationLinkProps) attrs() gx.Attrs {
	if !p.Active {
		return p.Attrs
	}
	return append(gx.Attrs{{Key: "aria-current", Value: "page"}}, p.Attrs...)
}

// attrs returns the link attributes of the previous link.
func (p PaginationPreviousProps) attrs() gx.Attrs {
	return append(gx.Attrs{{Key: "aria-label", Value: "Go to previous page"}}, p.Attrs...)
}

// attrs returns the link attributes of the next link.
func (p PaginationNextProps) attrs() gx.Attrs {
	return append(gx.Attrs{{Key: "aria-label", Value: "Go to next page"}}, p.Attrs...)
}
