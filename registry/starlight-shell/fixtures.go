package starlight

import "github.com/alternayte/gx"

// fixtureNav is the sidebar model of the fixtures: two open groups, with a
// nested group that starts closed.
var fixtureNav = Nav{Groups: []NavGroup{
	{Label: "Start", Items: []NavItem{
		{Label: "Introduction", Href: gx.URL("/start/"), Badge: "New"},
		{Label: "Install", Href: gx.URL("/start/install/")},
	}},
	{Label: "Reference", Items: []NavItem{
		{Label: "Configuration", Href: gx.URL("/reference/configuration/")},
		{Label: "Errors", Collapsed: true, Items: []NavItem{
			{Label: "GX1000", Href: gx.URL("/errors/GX1000/")},
		}},
	}},
}}

// fixtureSite is the site config of the fixtures.
var fixtureSite = Config{
	Title:    "Deedbox",
	Version:  "latest",
	Links:    []Link{{Label: "GitHub", Href: gx.URL("https://example.com/gx"), Icon: "github"}},
	EditBase: "https://example.com/edit/main/content/docs/",
}

// fixtureVersions is the site config of the fixture with a version select.
var fixtureVersions = Config{
	Title:   "Deedbox",
	Version: "latest",
	Versions: []Version{
		{Label: "latest", Href: gx.URL("https://example.com/")},
		{Label: "0.4", Href: gx.URL("https://example.com/0.4/")},
	},
}

// fixtureNext is the next page of the fixtures.
var fixtureNext = &NavItem{Label: "Install", Href: gx.URL("/start/install/")}

// fixturePrev is the previous page of the fixtures.
var fixturePrev = &NavItem{Label: "Introduction", Href: gx.URL("/start/")}

// fixtureHeadings are the headings of the fixtures.
var fixtureHeadings = []Heading{
	{Level: 2, Text: "Install", ID: "install"},
	{Level: 3, Text: "First page", ID: "first-page"},
}

// fixturePage is the page data of the fixtures.
var fixturePage = Page{
	Title:   "Introduction",
	Path:    "/start/",
	Updated: "Oct 2, 2026",
	Next:    fixtureNext,
	TOC:     fixtureHeadings,
}
