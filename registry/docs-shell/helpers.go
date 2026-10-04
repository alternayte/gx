package shell

import "strings"

// Active reports whether href is the current page or a parent of it
// (REQ-CNT-06). The sidebar marks active items on the server.
func Active(path string, href string) bool {
	if href == "" || path == "" {
		return false
	}
	if path == href {
		return true
	}
	return len(path) > len(href) && path[:len(href)] == href && path[len(href)-1] == '/'
}

// ActiveItem reports whether any descendant of item is active.
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

// HasTOC reports whether the page has headings to show.
func (p Page) HasTOC() bool {
	for _, h := range p.TOC {
		if h.Text != "" {
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

// ShownTOC returns the headings the table of contents shows.
func (p Page) ShownTOC() []Heading {
	depth := p.TocDepth()
	var out []Heading
	for _, h := range p.TOC {
		if h.Level >= 2 && h.Level <= depth && h.Text != "" {
			out = append(out, h)
		}
	}
	return out
}

// EditURL returns the edit-page link of the page (REQ-CNT-06). The page
// path maps to its Markdown file: "/start/" to "start.md".
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
