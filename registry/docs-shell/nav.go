// Package shell is the docs-shell registry block (REQ-CNT-06): a layout
// with a header, a sidebar, a table of contents, pagination and a 404, plus
// the client behaviours they need.
package shell

import "github.com/alternayte/gx"

// Nav is the sidebar model (REQ-CNT-06).
type Nav struct {
	Groups []NavGroup
}

// NavGroup is one sidebar section. A group with Collapsed starts closed.
type NavGroup struct {
	Label     string
	Badge     string
	Collapsed bool
	Items     []NavItem
}

// NavItem is one sidebar link. Items nest under a parent item.
type NavItem struct {
	Label string
	Href  gx.URL
	Badge string
	Items []NavItem
}

// Link is one header link: a social link or a plain link.
type Link struct {
	Label string
	Href  gx.URL
}

// Config is the site-wide shell data (REQ-CNT-06).
type Config struct {
	Title   string
	Version string
	Links   []Link
	// EditBase is the repository edit URL up to the file path, for example
	// https://github.com/acme/site/edit/main/content/docs/. Empty hides
	// the edit link.
	EditBase string
}

// Page is the shell data of one page (REQ-CNT-06).
type Page struct {
	Title   string
	// Path is the site path of the page, for example /start/.
	Path string
	// Section is the sidebar group label of the page.
	Section string
	// Updated is the last-updated text. Empty hides it.
	Updated string
	// Prev and Next are the previous and next sidebar items.
	Prev *NavItem
	Next *NavItem
	// TOC holds the page headings, in document order.
	TOC []Heading
	// Depth limits the table of contents to headings at or below this
	// level. Zero means 3.
	Depth int
}

// Heading is one table-of-contents entry (REQ-CNT-06).
type Heading struct {
	Level int
	Text  string
	ID    string
}
