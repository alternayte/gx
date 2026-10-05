//go:build gxdev

package gx

import (
	_ "embed"
	"sort"
)

// galleryNode renders the dev gallery: a head with a theme switch and a jump
// list of the components, then every fixture of every component, plus a
// missing entry for a component without fixtures (REQ-AI-03).
func galleryNode() Node {
	fixtures := Gallery()
	sort.Slice(fixtures, func(i, j int) bool {
		if fixtures[i].Component != fixtures[j].Component {
			return fixtures[i].Component < fixtures[j].Component
		}
		return fixtures[i].Name < fixtures[j].Name
	})
	var b Builder
	b.Add(galleryHead(fixtures))
	if len(fixtures) == 0 {
		b.Add(El("p", nil, Text("No components are registered. Call gx.SetGallery with gxdev_gallery.Fixtures().")))
	}
	for _, f := range fixtures {
		if f.Missing {
			b.Add(El("section", Attrs{
				{Key: "class", Value: "fixture missing", Kind: AttrText},
				{Key: "id", Value: galleryID(f), Kind: AttrText},
				{Key: "data-fixture", Value: f.Component, Kind: AttrText},
				{Key: "data-package", Value: f.Package, Kind: AttrText},
			},
				El("h2", nil, Text(f.Component)),
				El("p", nil, Text("missing fixtures: add "+f.Component+".fixtures.go")),
			))
			continue
		}
		var body Node = Frag()
		if f.Node != nil {
			body = f.Node()
		}
		b.Add(El("section", Attrs{
			{Key: "class", Value: "fixture", Kind: AttrText},
			{Key: "id", Value: galleryID(f), Kind: AttrText},
			{Key: "data-fixture", Value: f.Component + "-" + f.Name, Kind: AttrText},
			{Key: "data-package", Value: f.Package, Kind: AttrText},
		},
			El("h2", nil, Text(f.Component+" - "+f.Name)),
			El("div", Attrs{{Key: "class", Value: "fixture-body", Kind: AttrText}}, body),
		))
	}
	return b.Node()
}

// galleryID is the anchor of one gallery section.
func galleryID(f Fixture) string {
	if f.Missing || f.Name == "" {
		return "fx-" + f.Component
	}
	return "fx-" + f.Component + "-" + f.Name
}

// galleryHead renders the title, the theme switch and the jump list. The
// list has one link per component, to its first fixture.
func galleryHead(fixtures []Fixture) Node {
	var links Builder
	seen := map[string]bool{}
	for _, f := range fixtures {
		if seen[f.Component] {
			continue
		}
		seen[f.Component] = true
		links.Add(El("li", nil, El("a", Attrs{{Key: "href", Value: "#" + galleryID(f), Kind: AttrURL}}, Text(f.Component))))
	}
	theme := func(value, label string) Node {
		return El("button", Attrs{
			{Key: "type", Value: "button", Kind: AttrText},
			{Key: "data-gallery-theme", Value: value, Kind: AttrText},
		}, Text(label))
	}
	return El("header", Attrs{{Key: "class", Value: "gallery-head", Kind: AttrText}},
		El("h1", nil, Text("Gx gallery")),
		El("div", Attrs{
			{Key: "class", Value: "gallery-theme", Kind: AttrText},
			{Key: "role", Value: "group", Kind: AttrText},
			{Key: "aria-label", Value: "Theme", Kind: AttrText},
		}, theme("light", "Light"), theme("dark", "Dark"), theme("auto", "System")),
		El("details", Attrs{{Key: "class", Value: "gallery-jump", Kind: AttrText}},
			El("summary", nil, Text("Components")),
			El("ul", nil, links.Node()),
		),
	)
}

// galleryThemeJS switches the theme class of the gallery document. The
// page is dev only, so the script is inline.
const galleryThemeJS = `document.addEventListener("click",function(e){var b=e.target.closest("[data-gallery-theme]");if(!b)return;var t=b.getAttribute("data-gallery-theme"),c=document.documentElement.classList;c.toggle("dark",t==="dark");c.toggle("light",t==="light")})`

// galleryPageHTML wraps the fixtures in a document that uses the shadcn
// tokens, so a pasted theme changes it (REQ-AI-03, REQ-STY-03).
func galleryPageHTML() Node {
	return Frag(
		Raw("<!DOCTYPE html>"),
		El("html", Attrs{{Key: "lang", Value: "en", Kind: AttrText}},
			El("head", nil,
				El("meta", Attrs{{Key: "charset", Value: "utf-8", Kind: AttrText}}),
				El("meta", Attrs{{Key: "viewport", Value: "width=device-width, initial-scale=1", Kind: AttrText}}),
				El("title", nil, Text("Gx gallery")),
				El("style", nil, Raw(galleryCSS)),
			),
			El("body", nil,
				galleryNode(),
				El("script", nil, Raw(SafeHTML(galleryThemeJS))), //gx:trusted a constant dev-only script
			),
		),
	)
}

// galleryFixture finds one fixture by component, name and, when two
// packages hold the component name, package.
func galleryFixture(component, name, pkg string) (Fixture, bool) {
	for _, f := range Gallery() {
		if f.Missing || f.Component != component || f.Name != name {
			continue
		}
		if pkg != "" && f.Package != pkg {
			continue
		}
		return f, true
	}
	return Fixture{}, false
}

// galleryFixtureHTML is the document of one fixture: the tokens and the
// stylesheet of the gallery, and the fixture alone in #gx-fixture. The dev
// MCP server renders and audits it (REQ-AI-04). theme is "light", "dark" or
// "" for the system preference.
func galleryFixtureHTML(f Fixture, theme string) Node {
	var body Node = Frag()
	if f.Node != nil {
		body = f.Node()
	}
	html := Attrs{{Key: "lang", Value: "en", Kind: AttrText}}
	if theme == "light" || theme == "dark" {
		html = append(html, Attr{Key: "class", Value: theme, Kind: AttrText})
	}
	return Frag(
		Raw("<!DOCTYPE html>"),
		El("html", html,
			El("head", nil,
				El("meta", Attrs{{Key: "charset", Value: "utf-8", Kind: AttrText}}),
				El("meta", Attrs{{Key: "viewport", Value: "width=device-width, initial-scale=1", Kind: AttrText}}),
				El("title", nil, Text(f.Component+" - "+f.Name)),
				El("style", nil, Raw(galleryCSS)),
			),
			El("body", nil,
				El("main", Attrs{
					{Key: "id", Value: "gx-fixture", Kind: AttrText},
					{Key: "class", Value: "fixture-body", Kind: AttrText},
					{Key: "data-fixture", Value: f.Component + "-" + f.Name, Kind: AttrText},
					{Key: "data-package", Value: f.Package, Kind: AttrText},
				}, body),
			),
		),
	)
}

// galleryShellCSS is the gallery shell style: token names match shadcn, and
// dark mode follows a .dark class or the system preference (REQ-STY-03).
//
//go:embed gallery.css
var galleryShellCSS string

// galleryCSS is the gallery shell style the page inlines.
var galleryCSS = SafeHTML(galleryShellCSS)
