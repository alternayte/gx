package main

import (
	"sort"
	"strings"

	"github.com/alternayte/gx"
	"github.com/alternayte/gx/content"
	"shellapp/ui/shell"
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
}

// Docs is the docs collection of the parity app.
var Docs = gx.Collection[DocMeta]("content/docs")

var site = shell.Config{
	Title:    "Deedbox docs",
	Version:  "v0.42.0",
	Links:    []shell.Link{{Label: "GitHub", Href: gx.URL("https://example.com/deedbox")}},
	EditBase: "https://example.com/deedbox/edit/main/content/docs/",
}

// Routes serves one page per entry.
var Routes = gx.Collect(gx.ContentEntries(Docs, view))

// view renders one entry inside the docs shell.
func view(e gx.Entry[DocMeta]) gx.Node {
	body, err := content.Body(e.Body)
	if err != nil {
		body = gx.Text("body error: " + err.Error())
	}
	if e.Meta.Template == "splash" {
		return shell.Splash(shell.SplashProps{
			Title:   e.Meta.Title,
			Tagline: e.Meta.Description,
			Actions: gx.Frag(
				gx.El("a", gx.Attrs{{Key: "href", Value: "/start/", Kind: gx.AttrURL}}, gx.Text("Get started")),
			),
			Children: body,
		})
	}
	return shell.Shell(shell.ShellProps{
		Site:     site,
		Nav:      nav(),
		Page:     pageFor(e),
		Children: body,
	})
}

// titleCase upper-cases the first letter of a slug segment.
func titleCase(s string) string {
	s = strings.ReplaceAll(s, "-", " ")
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// groupOf returns the sidebar group of one entry: the explicit frontmatter
// group, or the first slug segment.
func groupOf(e gx.Entry[DocMeta]) string {
	if e.Meta.SidebarGroup != "" {
		return e.Meta.SidebarGroup
	}
	if i := strings.IndexByte(e.Slug, '/'); i > 0 {
		return titleCase(e.Slug[:i])
	}
	return "Start"
}

// navNode is one node of the sidebar tree.
type navNode struct {
	slug     string
	title    string
	badge    string
	order    int
	children []*navNode
}

// nav builds the sidebar from the entry slugs: the first slug segment is a
// group, nested segments are nested items, entries order by Order, and a
// splash entry stays out (REQ-CNT-06).
func nav() shell.Nav {
	type group struct {
		node      *navNode
		collapsed bool
	}
	groups := map[string]*group{}
	items := map[string]*navNode{}
	var groupOrder []string
	for _, e := range Docs.Entries() {
		if e.Meta.Template == "splash" {
			continue
		}
		parts := strings.Split(e.Slug, "/")
		key := parts[0]
		g, ok := groups[key]
		if !ok {
			g = &group{node: &navNode{slug: key, title: groupLabel(key), order: 1000}}
			groups[key] = g
			groupOrder = append(groupOrder, key)
		}
		parent := g.node
		var leaf *navNode
		if len(parts) == 1 {
			leaf = &navNode{slug: e.Slug, order: 1000}
			parent.children = append(parent.children, leaf)
		} else {
			for i := 1; i < len(parts); i++ {
				full := strings.Join(parts[:i+1], "/")
				node, ok := items[full]
				if !ok {
					node = &navNode{slug: full, title: titleCase(parts[i]), order: 1000}
					items[full] = node
					parent.children = append(parent.children, node)
				}
				parent = node
			}
			leaf = parent
		}
		leaf.title = e.Meta.Title
		leaf.badge = e.Meta.Badge
		leaf.order = e.Meta.Order
		if e.Meta.Collapsed {
			g.collapsed = true
		}
	}
	sort.SliceStable(groupOrder, func(i, j int) bool {
		return groupRank(groupOrder[i]) < groupRank(groupOrder[j])
	})
	var out shell.Nav
	for _, key := range groupOrder {
		g := groups[key]
		sortTree(g.node)
		ng := shell.NavGroup{Label: g.node.title, Collapsed: g.collapsed}
		for _, item := range g.node.children {
			ng.Items = append(ng.Items, navItem(item))
		}
		out.Groups = append(out.Groups, ng)
	}
	return out
}

// groupRank orders the sidebar groups; unknown groups come last in slug
// order.
func groupRank(key string) int {
	switch strings.ToLower(key) {
	case "start":
		return 0
	case "guides":
		return 1
	case "errors":
		return 2
	default:
		return 100
	}
}

// groupLabel returns the shown label of a top-level folder.
func groupLabel(slug string) string {
	if slug == "start" {
		return "Start"
	}
	return titleCase(slug)
}

// sortTree orders siblings by Order and then by slug.
func sortTree(n *navNode) {
	sort.SliceStable(n.children, func(i, j int) bool {
		a, b := n.children[i], n.children[j]
		if a.order != b.order {
			return a.order < b.order
		}
		return a.slug < b.slug
	})
	for _, c := range n.children {
		sortTree(c)
	}
}

// pageHref returns the site path of one entry slug.
func pageHref(slug string) gx.URL {
	if slug == "index" {
		return gx.URL("/")
	}
	return gx.URL("/" + slug + "/")
}

// navItem converts one tree node to a sidebar item.
func navItem(n *navNode) shell.NavItem {
	item := shell.NavItem{Label: n.title, Href: pageHref(n.slug), Badge: n.badge}
	for _, c := range n.children {
		item.Items = append(item.Items, navItem(c))
	}
	return item
}

// flat returns the entries in sidebar order.
func flat() []gx.Entry[DocMeta] {
	entries := Docs.Entries()
	groups := map[string][]gx.Entry[DocMeta]{}
	var order []string
	for _, e := range entries {
		g := groupOf(e)
		if _, ok := groups[g]; !ok {
			order = append(order, g)
		}
		groups[g] = append(groups[g], e)
	}
	sort.SliceStable(order, func(i, j int) bool {
		if groupRank(order[i]) != groupRank(order[j]) {
			return groupRank(order[i]) < groupRank(order[j])
		}
		return order[i] < order[j]
	})
	var out []gx.Entry[DocMeta]
	for _, g := range order {
		items := groups[g]
		sort.SliceStable(items, func(i, j int) bool {
			a, b := items[i], items[j]
			if a.Meta.Order != b.Meta.Order {
				return a.Meta.Order < b.Meta.Order
			}
			return a.Slug < b.Slug
		})
		out = append(out, items...)
	}
	return out
}

// pageFor builds the shell data of one entry.
func pageFor(e gx.Entry[DocMeta]) shell.Page {
	p := shell.Page{
		Title:   e.Meta.Title,
		Path:    string(pageHref(e.Slug)),
		Section: groupOf(e),
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
			p.Prev = &shell.NavItem{Label: all[i-1].Meta.Title, Href: pageHref(all[i-1].Slug)}
		}
		if i+1 < len(all) {
			p.Next = &shell.NavItem{Label: all[i+1].Meta.Title, Href: pageHref(all[i+1].Slug)}
		}
		break
	}
	return p
}
