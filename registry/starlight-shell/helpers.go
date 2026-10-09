package starlight

import (
	"strings"

	"github.com/alternayte/gx"
	"github.com/alternayte/gx/registry/docs"
)

// icon renders one built-in icon of the docs kit at the size of the text.
func icon(name docs.IconName, class string) gx.Node {
	return gx.Icon(string(docs.IconBody(name)), gx.IconProps{Class: gx.Cx("sl-icon", class)})
}

// Active reports whether href is the current page.
func Active(path string, href string) bool {
	return href != "" && path == href
}

// ActiveItem reports whether item or one of its descendants is the current
// page.
func ActiveItem(path string, item NavItem) bool {
	if Active(path, string(item.Href)) {
		return true
	}
	for _, child := range item.Items {
		if ActiveItem(path, child) {
			return true
		}
	}
	return false
}

// GroupOpen reports whether a sidebar group starts open: it is not
// collapsed, or it holds the current page.
func GroupOpen(path string, collapsed bool, items []NavItem) bool {
	if !collapsed {
		return true
	}
	for _, item := range items {
		if ActiveItem(path, item) {
			return true
		}
	}
	return false
}

// TocDepth returns the heading depth limit of the page.
func (p Page) TocDepth() int {
	if p.Depth <= 0 {
		return 3
	}
	return p.Depth
}

// ShownTOC returns the headings that the table of contents shows. A splash
// page has none.
func (p Page) ShownTOC() []Heading {
	if p.Splash {
		return nil
	}
	depth := p.TocDepth()
	var out []Heading
	for _, h := range p.TOC {
		if h.Level >= 2 && h.Level <= depth && h.Text != "" {
			out = append(out, h)
		}
	}
	return out
}

// RootClass returns the classes of the html element. The theme sets the
// widths of the layout from them.
func RootClass(p Page) string {
	if p.Splash {
		return ""
	}
	// A page with a sidebar always has the entry "Overview" in its table
	// of contents.
	return "sl-has-sidebar sl-has-toc"
}

// EditURL returns the edit link of the page. The page path maps to its
// Markdown file: "/start/" to "start.md".
func (c Config) EditURL(p Page) string {
	if c.EditBase == "" || p.Path == "" {
		return ""
	}
	base := c.EditBase
	if base[len(base)-1] != '/' {
		base += "/"
	}
	path := strings.TrimPrefix(p.Path, "/")
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		path = "index"
	}
	return base + path + ".md"
}
