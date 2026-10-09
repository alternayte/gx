// Package starlight is the starlight-shell registry block: the page
// structure of the default Starlight theme. It has a fixed header, a
// sidebar, a table of contents, a hero, page links and a search dialog.
// The `starlight` theme styles it; the `docs` kit holds its icons.
package starlight

import (
	"github.com/alternayte/gx"
	"github.com/alternayte/gx/registry/docs"
)

// Nav is the sidebar model.
type Nav struct {
	Groups []NavGroup
}

// NavGroup is one top-level sidebar section. A group with Collapsed starts
// closed, unless it holds the current page.
type NavGroup struct {
	Label     string
	Badge     string
	Collapsed bool
	Items     []NavItem
}

// NavItem is one sidebar link. An item with Items is a nested group; a
// nested group with Collapsed starts closed, unless it holds the current
// page.
type NavItem struct {
	Label     string
	Href      gx.URL
	Badge     string
	Collapsed bool
	Items     []NavItem
}

// Link is one social link of the header. Icon is the name of a built-in
// icon of the docs kit, for example "github".
type Link struct {
	Label string
	Href  gx.URL
	Icon  docs.IconName
}

// Version is one version of the docs: its label and the address of its
// site.
type Version struct {
	Label string
	Href  gx.URL
}

// Config is the site-wide shell data.
type Config struct {
	Title string
	// Version is the label of the version that this site shows. The header
	// shows it after the title. Empty hides it.
	Version string
	// Versions lists the versions of the docs. With two or more, the
	// header shows a select in place of the label, and a choice opens the
	// site of that version.
	Versions []Version
	Links    []Link
	// EditBase is the repository edit URL up to the file path, for example
	// https://github.com/acme/site/edit/main/content/docs/. Empty hides
	// the edit link.
	EditBase string
}

// Page is the shell data of one page.
type Page struct {
	Title string
	// Path is the site path of the page, for example /start/.
	Path string
	// Splash is true for a page with no sidebar and no table of contents,
	// such as a home page.
	Splash bool
	// Updated is the date of the last change. Empty hides it.
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

// Heading is one table-of-contents entry.
type Heading struct {
	Level int
	Text  string
	ID    string
}
