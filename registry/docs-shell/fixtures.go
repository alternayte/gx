package shell

import "github.com/alternayte/gx"

// fixtureNav is the sidebar model of the shell fixtures (REQ-CNT-06).
var fixtureNav = Nav{Groups: []NavGroup{
	{Label: "Start", Items: []NavItem{
		{Label: "Introduction", Href: gx.URL("/start/"), Badge: "New"},
	}},
	{Label: "Guides", Badge: "12", Items: []NavItem{
		{Label: "Routing", Href: gx.URL("/guides/routing/"), Items: []NavItem{
			{Label: "Pages", Href: gx.URL("/guides/routing/pages/")},
		}},
	}},
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

// fixturePage is the page data of the shell fixtures.
var fixturePage = Page{
	Title:   "Introduction",
	Path:    "/start/",
	Section: "Start",
	Updated: "Oct 2, 2026",
	Prev:    nil,
	Next:    &NavItem{Label: "Routing", Href: gx.URL("/guides/routing/")},
	TOC: []Heading{
		{Level: 2, Text: "Install", ID: "install"},
		{Level: 3, Text: "First page", ID: "first-page"},
	},
}
