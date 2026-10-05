package shell

import "github.com/alternayte/gx"

// fixtureRouting is the nested item of the shell fixtures.
var fixtureRouting = NavItem{Label: "Routing", Href: gx.URL("/guides/routing/"), Items: []NavItem{
	{Label: "Pages", Href: gx.URL("/guides/routing/pages/")},
}}

// fixtureStart and fixtureGuides are the open groups of the shell fixtures.
var fixtureStart = NavGroup{Label: "Start", Items: []NavItem{
	{Label: "Introduction", Href: gx.URL("/start/"), Badge: "New"},
}}

var fixtureGuides = NavGroup{Label: "Guides", Badge: "12", Items: []NavItem{fixtureRouting}}

// fixtureNav is the sidebar model of the shell fixtures (REQ-CNT-06).
var fixtureNav = Nav{Groups: []NavGroup{
	fixtureStart,
	fixtureGuides,
	{Label: "Errors", Collapsed: true, Items: []NavItem{
		{Label: "GX1000", Href: gx.URL("/errors/GX1000/")},
	}},
}}

// fixtureSite is the site config of the shell fixtures.
var fixtureSite = Config{
	Title:    "Deedbox docs",
	Version:  "v0.1.0",
	Links:    []Link{{Label: "GitHub", Href: gx.URL("https://example.com/gx")}},
	EditBase: "https://example.com/edit/main/content/docs/",
}

// fixtureNext is the next page of the shell fixtures.
var fixtureNext = &NavItem{Label: "Routing", Href: gx.URL("/guides/routing/")}

// fixtureHeadings are the headings of the shell fixtures. Each one is in
// the default depth of the table of contents.
var fixtureHeadings = []Heading{
	{Level: 2, Text: "Install", ID: "install"},
	{Level: 3, Text: "First page", ID: "first-page"},
}

// fixturePage is the page data of the shell fixtures.
var fixturePage = Page{
	Title:   "Introduction",
	Path:    "/start/",
	Section: "Start",
	Updated: "Oct 2, 2026",
	Prev:    nil,
	Next:    fixtureNext,
	TOC:     fixtureHeadings,
}
