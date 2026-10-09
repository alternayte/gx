// Package docs is the Deedbox docs slice: the content collection, the
// sidebar and the page view (REQ-CNT-14). The content is a port of the
// Deedbox docs (MIT), converted with `gx import starlight`.
package docs

import (
	"encoding/json"
	"os"
	"sort"
	"strings"

	"github.com/alternayte/gx"
	"github.com/alternayte/gx/content"
	"github.com/alternayte/gx/registry/docs"
	shell "github.com/alternayte/gx/registry/starlight-shell"
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

// Site is the shell configuration (REQ-CNT-14). The Deedbox site has one
// version, so the header shows its label; a second entry in Versions makes
// the label a select.
var Site = shell.Config{
	Title:    "Deedbox",
	Version:  "latest",
	Versions: []shell.Version{{Label: "latest", Href: gx.URL("https://deedbox-docs.pages.dev")}},
	Links:    []shell.Link{{Label: "GitHub", Href: gx.URL("https://github.com/alternayte/deedbox"), Icon: "github"}},
	EditBase: "https://github.com/alternayte/deedbox/edit/main/site/src/content/docs/",
}

// beaconToken is the token of Cloudflare Web Analytics. The deploy sets it;
// the analytics are cookie-free and off without it.
var beaconToken = os.Getenv("PUBLIC_CF_BEACON_TOKEN")

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

// View renders one entry inside the Starlight shell.
func View(e gx.Entry[DocMeta]) gx.Node {
	props := shell.ShellProps{
		Site:     Site,
		Nav:      Nav(),
		Page:     pageFor(e),
		Children: DocsBody(e),
	}
	if e.Meta.Template == "splash" {
		props.Page.Splash = true
		props.Hero = hero(e)
	}
	return gx.Frag(shell.Shell(props), beacon())
}

// hero renders the hero of the home page. `gx import starlight` does not
// convert the hero actions of the frontmatter, so they are here.
func hero(e gx.Entry[DocMeta]) gx.Node {
	return shell.Hero(shell.HeroProps{
		Title:   e.Meta.Title,
		Tagline: e.Meta.Description,
		Actions: gx.Frag(
			docs.LinkButton(docs.LinkButtonProps{
				Href:     gx.URL("/tutorials/first-stream/"),
				Variant:  docs.ButtonPrimary,
				Icon:     "right-arrow",
				Children: gx.Text("Write your first stream"),
			}),
			docs.LinkButton(docs.LinkButtonProps{
				Href:     gx.URL("/tutorials/existing-ef-core-app/"),
				Variant:  docs.ButtonMinimal,
				Children: gx.Text("Add Deedbox to an EF Core app"),
			}),
		),
	})
}

// beacon renders the script of Cloudflare Web Analytics, as the head
// option of the Starlight config does. With no token it renders nothing.
func beacon() gx.Node {
	if beaconToken == "" {
		return gx.Text("")
	}
	data, _ := json.Marshal(map[string]string{"token": beaconToken})
	return gx.El("script", gx.Attrs{
		gx.Bool("defer", true),
		{Key: "src", Value: "https://static.cloudflareinsights.com/beacon.min.js", Kind: gx.AttrURL},
		{Key: "data-cf-beacon", Value: string(data), Kind: gx.AttrText},
	})
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
			nested := shell.NavItem{Label: "Errors", Collapsed: true}
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
