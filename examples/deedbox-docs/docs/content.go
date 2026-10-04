// Package docs is the Deedbox docs slice: the content collection, the
// sidebar and the page view (REQ-CNT-14). The content is a port of the
// Deedbox docs (MIT), converted with `gx import starlight`.
package docs

import (
	"sort"
	"strings"

	"github.com/alternayte/gx"
	"github.com/alternayte/gx/content"
	"github.com/alternayte/gx/registry/docs"
	"github.com/alternayte/gx/registry/docs-shell"
)

// DocMeta is the frontmatter of one docs page (REQ-CNT-06).
type DocMeta struct {
	Title        string `yaml:"title"`
	Description  string `yaml:"description"`
	Order        int    `yaml:"order"`
	Badge        string `yaml:"badge"`
	SidebarGroup string `yaml:"sidebarGroup"`
	Collapsed    bool   `yaml:"collapsed"`
	Updated      string `yaml:"updated"`
	Template     string `yaml:"template"`
	LLMS         string `yaml:"llms"`
}

// Docs is the Deedbox docs collection.
var Docs = gx.Collection[DocMeta]("content/docs").Components(
	docs.Aside, docs.Tabs, docs.TabItem, docs.Steps, docs.Card, docs.CardGrid,
	docs.LinkCard, docs.LinkButton, docs.Badge, docs.FileTree, docs.Code, docs.LLMSkip,
)

// Site is the shell configuration (REQ-CNT-14).
var Site = shell.Config{
	Title:    "Deedbox",
	Version:  "v1.0.0",
	Links:    []shell.Link{{Label: "GitHub", Href: gx.URL("https://github.com/alternayte/deedbox")}},
	EditBase: "https://github.com/alternayte/deedbox/edit/main/site/src/content/docs/",
}

// groupOrder is the sidebar order of the Starlight config (REQ-CNT-06).
var groupOrder = []string{"Tutorials", "How-to guides", "Concepts", "Reference", "Operations", "Errors"}

// Routes serves one page per entry and the llms.txt metadata (REQ-CNT-08).
var Routes = gx.Collect(gx.ContentEntries(Docs, View).LLMS(gx.LLMSOptions[DocMeta]{
	Site:        "Deedbox",
	Summary:     "Event-source part of your app. Postgres or SQL Server. EF Core, Dapper, or neither.",
	Title:       func(m DocMeta) string { return m.Title },
	Description: func(m DocMeta) string { return m.Description },
	Skip:        func(m DocMeta) bool { return m.LLMS == "skip" },
}))

// View renders one entry inside the docs shell.
func View(e gx.Entry[DocMeta]) gx.Node {
	body := DocsBody(e)
	if e.Meta.Template == "splash" {
		return splash(e, body)
	}
	return shell.Shell(shell.ShellProps{
		Site:     Site,
		Nav:      Nav(),
		Page:     pageFor(e),
		Children: body,
	})
}

// splash renders the home page without the sidebar (REQ-CNT-14).
func splash(e gx.Entry[DocMeta], body gx.Node) gx.Node {
	heading := shell.Splash(shell.SplashProps{
		Title:   e.Meta.Title,
		Tagline: e.Meta.Description,
		Actions: gx.Frag(
			docs.LinkButton(docs.LinkButtonProps{
				Href:     gx.URL("/tutorials/first-stream/"),
				Children: gx.Text("Write your first stream"),
			}),
			docs.LinkButton(docs.LinkButtonProps{
				Href:     gx.URL("/tutorials/existing-ef-core-app/"),
				Variant:  docs.ButtonSecondary,
				Children: gx.Text("Add Deedbox to an EF Core app"),
			}),
		),
		Children: body,
	})
	return gx.Frag(
		shell.Header(shell.HeaderProps{Site: Site}),
		gx.El("main", gx.Attrs{{Key: "id", Value: "gx-main", Kind: gx.AttrText}}, heading),
		shell.SearchDialog(shell.SearchDialogProps{}),
	)
}

// entryHref returns the site path of one entry slug.
func entryHref(slug string) gx.URL {
	if slug == "index" || slug == "" {
		return gx.URL("/")
	}
	return gx.URL("/" + slug + "/")
}

// Nav builds the sidebar: the Starlight groups, with the error reference
// nested under Reference (REQ-CNT-06).
func Nav() shell.Nav {
	entries := Docs.Entries()
	groups := map[string][]gx.Entry[DocMeta]{}
	for _, e := range entries {
		groups[e.Meta.SidebarGroup] = append(groups[e.Meta.SidebarGroup], e)
	}
	var out shell.Nav
	for _, label := range groupOrder {
		if label == "Errors" {
			continue // nested under Reference
		}
		items := groups[label]
		if len(items) == 0 {
			continue
		}
		sortEntries(items)
		group := shell.NavGroup{Label: label, Collapsed: collapsedOf(items)}
		for _, e := range items {
			group.Items = append(group.Items, navItem(e))
		}
		if label == "Reference" {
			errorsItems := groups["Errors"]
			sortEntries(errorsItems)
			nested := shell.NavItem{Label: "Errors", Badge: "37"}
			for _, e := range errorsItems {
				nested.Items = append(nested.Items, navItem(e))
			}
			if len(nested.Items) > 0 {
				group.Items = append(group.Items, nested)
			}
		}
		out.Groups = append(out.Groups, group)
	}
	return out
}

// navItem converts one entry to a sidebar item.
func navItem(e gx.Entry[DocMeta]) shell.NavItem {
	return shell.NavItem{Label: e.Meta.Title, Href: entryHref(e.Slug), Badge: e.Meta.Badge}
}

// sortEntries orders one sidebar group: explicit order first, then slug.
func sortEntries(entries []gx.Entry[DocMeta]) {
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if a.Meta.Order != b.Meta.Order {
			if a.Meta.Order == 0 {
				return false
			}
			if b.Meta.Order == 0 {
				return true
			}
			return a.Meta.Order < b.Meta.Order
		}
		return a.Slug < b.Slug
	})
}

// collapsedOf reports whether a sidebar group starts closed.
func collapsedOf(entries []gx.Entry[DocMeta]) bool {
	for _, e := range entries {
		if e.Meta.Collapsed {
			return true
		}
	}
	return false
}

// pageFor builds the shell data of one entry.
func pageFor(e gx.Entry[DocMeta]) shell.Page {
	p := shell.Page{
		Title:   e.Meta.Title,
		Path:    string(entryHref(e.Slug)),
		Section: e.Meta.SidebarGroup,
		Updated: e.Meta.Updated,
	}
	for _, h := range content.Headings(e.Body) {
		p.TOC = append(p.TOC, shell.Heading{Level: h.Level, Text: h.Text, ID: h.ID})
	}
	all := flat()
	for i, other := range all {
		if other.Slug != e.Slug {
			continue
		}
		if i > 0 {
			p.Prev = &shell.NavItem{Label: all[i-1].Meta.Title, Href: entryHref(all[i-1].Slug)}
		}
		if i+1 < len(all) {
			p.Next = &shell.NavItem{Label: all[i+1].Meta.Title, Href: entryHref(all[i+1].Slug)}
		}
		break
	}
	return p
}

// flat returns the entries in sidebar order.
func flat() []gx.Entry[DocMeta] {
	var out []gx.Entry[DocMeta]
	for _, group := range Nav().Groups {
		for _, item := range group.Items {
			if len(item.Items) > 0 {
				for _, child := range item.Items {
					if e, ok := entryFor(child.Href); ok {
						out = append(out, e)
					}
				}
				continue
			}
			if e, ok := entryFor(item.Href); ok {
				out = append(out, e)
			}
		}
	}
	return out
}

// entryFor finds the entry of one sidebar link.
func entryFor(href gx.URL) (gx.Entry[DocMeta], bool) {
	slug := strings.Trim(string(href), "/")
	if slug == "" {
		slug = "index"
	}
	return Docs.Get(slug)
}
