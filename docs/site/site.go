// Package site is the docs site slice: the content collection, the sidebar,
// the page view and the live example previews (REQ-DOC-02). `just docs-gen`
// writes the component pages under content/ from the registry source.
package site

import (
	"sort"
	"strings"

	"github.com/alternayte/gx"
	"github.com/alternayte/gx/content"
	"github.com/alternayte/gx/docs/registry"
	"github.com/alternayte/gx/docs/site/route"
	"github.com/alternayte/gx/registry/docs"
	"github.com/alternayte/gx/registry/docs-shell"
)

// Meta is the frontmatter of one page.
type Meta struct {
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
	// Group is the sidebar group of a component page.
	Group string `yaml:"group"`
	// Item is the registry item of a component page.
	Item string `yaml:"item"`
	// Section is the sidebar section of a page that is not a component
	// page: Start, Tutorial, Guides, Reference, Diagnostics or Compare.
	Section string `yaml:"section"`
	// Order sorts the pages of one section. A page with no order comes
	// last, in title order.
	Order int `yaml:"order"`
	// Code is the diagnostic code of a diagnostic page.
	Code string `yaml:"code"`
}

// sections is the order of the sidebar sections before the components.
var sections = []string{"Start", "Tutorial", "Guides", "Reference", "Compare", "Diagnostics"}

// Pages is the content collection of the site.
var Pages = gx.Collection[Meta]("content").Components(
	docs.Aside, docs.Tabs, docs.TabItem, docs.Card, docs.CardGrid, docs.LinkCard,
	Example, IconGrid,
)

// Site is the shell configuration.
var Site = shell.Config{
	Title: "Gx",
	Links: []shell.Link{{Label: "GitHub", Href: gx.URL("https://github.com/alternayte/gx")}},
}

// summary is the one-line description of the site.
const summary = "Gx is a Go framework for server-rendered web apps on Datastar."

// indexSlug is the slug of the components index page.
const indexSlug = "components"

// PreviewPage renders one live example alone, for the frame of a component
// page. The static export writes one page per example.
var PreviewPage = gx.Page(
	func(c *gx.Ctx, in route.Preview) (PreviewProps, error) {
		it, ok := registry.Find(in.Item)
		if !ok {
			return PreviewProps{}, gx.NotFound()
		}
		ex, ok := it.Example(in.Example)
		if !ok {
			return PreviewProps{}, gx.NotFound()
		}
		node := ex.Node()
		if ex.Toast {
			node = ToastDemo(ToastDemoProps{Children: node})
		}
		return PreviewProps{Title: it.Title + ": " + ex.Title, Block: it.Block, Children: node}, nil
	},
	Preview,
).Static(func() ([]route.Preview, error) {
	var out []route.Preview
	for _, it := range registry.Items {
		for _, ex := range it.Examples {
			out = append(out, route.Preview{Item: it.Name, Example: ex.Name})
		}
	}
	return out, nil
})

// Routes serves the previews, one page per content entry and the llms.txt
// metadata.
var Routes = gx.Collect(PreviewPage, gx.ContentEntries(Pages, View).LLMS(gx.LLMSOptions[Meta]{
	Site:        Site.Title,
	Summary:     summary,
	Title:       func(m Meta) string { return m.Title },
	Description: func(m Meta) string { return m.Description },
}))

// View renders one entry inside the docs shell. The home page is a splash
// without the sidebar.
func View(e gx.Entry[Meta]) gx.Node {
	body := PagesBody(e)
	head := gx.Head(gx.HeadProps{Title: e.Meta.Title})
	if e.Slug == "index" {
		return gx.Frag(
			head,
			shell.Header(shell.HeaderProps{Site: Site}),
			gx.El("main", gx.Attrs{{Key: "id", Value: "gx-main"}, {Key: "class", Value: "mx-auto max-w-3xl px-4"}},
				shell.Splash(shell.SplashProps{
					Title:   e.Meta.Title,
					Tagline: e.Meta.Description,
					Actions: docs.LinkButton(docs.LinkButtonProps{Href: entryHref(indexSlug), Children: gx.Text("Browse the components")}),
				}),
				gx.El("article", gx.Attrs{{Key: "class", Value: "gx-content"}, {Key: "data-pagefind-body", Value: ""}}, body),
			),
			shell.SearchDialog(shell.SearchDialogProps{}),
		)
	}
	return gx.Frag(head, shell.Shell(shell.ShellProps{
		Site: Site,
		Nav:  Nav(),
		Page: pageFor(e),
		Children: gx.Frag(
			PageHeader(PageHeaderProps{Title: e.Meta.Title, Description: e.Meta.Description}),
			body,
		),
	}))
}

// entryHref returns the site path of one entry slug.
func entryHref(slug string) gx.URL {
	if slug == "index" || slug == "" {
		return gx.URL("/")
	}
	return gx.URL("/" + slug + "/")
}

// Nav builds the sidebar: the index, then one group per kind of item, each
// in title order.
func Nav() shell.Nav {
	var nav shell.Nav
	bySection := map[string][]gx.Entry[Meta]{}
	byGroup := map[string][]gx.Entry[Meta]{}
	for _, e := range Pages.Entries() {
		switch {
		case e.Meta.Item != "":
			byGroup[e.Meta.Group] = append(byGroup[e.Meta.Group], e)
		case e.Meta.Section != "":
			bySection[e.Meta.Section] = append(bySection[e.Meta.Section], e)
		}
	}
	for _, label := range sections {
		entries := bySection[label]
		if len(entries) == 0 {
			continue
		}
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
			return strings.ToLower(a.Meta.Title) < strings.ToLower(b.Meta.Title)
		})
		// The diagnostics are a long list that a reader opens from a link.
		group := shell.NavGroup{Label: label, Collapsed: label == "Diagnostics"}
		for _, e := range entries {
			group.Items = append(group.Items, shell.NavItem{Label: e.Meta.Title, Href: entryHref(e.Slug)})
		}
		nav.Groups = append(nav.Groups, group)
	}
	nav.Groups = append(nav.Groups, shell.NavGroup{
		Label: "Components",
		Items: []shell.NavItem{{Label: "All components", Href: entryHref(indexSlug)}},
	})
	for _, label := range registry.Groups {
		entries := byGroup[label]
		if len(entries) == 0 {
			continue
		}
		sort.Slice(entries, func(i, j int) bool {
			a, b := strings.ToLower(entries[i].Meta.Title), strings.ToLower(entries[j].Meta.Title)
			if a != b {
				return a < b
			}
			return entries[i].Slug < entries[j].Slug
		})
		group := shell.NavGroup{Label: label}
		for _, e := range entries {
			group.Items = append(group.Items, shell.NavItem{Label: e.Meta.Title, Href: entryHref(e.Slug)})
		}
		nav.Groups = append(nav.Groups, group)
	}
	return nav
}

// pageFor builds the shell data of one entry: the table of contents and
// the previous and next pages in sidebar order.
func pageFor(e gx.Entry[Meta]) shell.Page {
	section := e.Meta.Group
	if e.Meta.Section != "" {
		section = e.Meta.Section
	}
	p := shell.Page{Title: e.Meta.Title, Path: string(entryHref(e.Slug)), Section: section}
	for _, h := range content.Headings(e.Body) {
		p.TOC = append(p.TOC, shell.Heading{Level: h.Level, Text: h.Text, ID: h.ID})
	}
	var flat []shell.NavItem
	for _, group := range Nav().Groups {
		flat = append(flat, group.Items...)
	}
	for i, item := range flat {
		if string(item.Href) != p.Path {
			continue
		}
		if i > 0 {
			p.Prev = &flat[i-1]
		}
		if i+1 < len(flat) {
			p.Next = &flat[i+1]
		}
		break
	}
	return p
}
