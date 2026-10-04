//go:build gxdev

package gx

import (
	_ "embed"
	"sort"
)

// galleryNode renders the fixture list of the dev gallery: every fixture of
// every component, plus a missing entry for a component without fixtures
// (REQ-AI-03).
func galleryNode() Node {
	fixtures := Gallery()
	sort.Slice(fixtures, func(i, j int) bool {
		if fixtures[i].Component != fixtures[j].Component {
			return fixtures[i].Component < fixtures[j].Component
		}
		return fixtures[i].Name < fixtures[j].Name
	})
	var b Builder
	b.Add(El("h1", nil, Text("Gx gallery")))
	if len(fixtures) == 0 {
		b.Add(El("p", nil, Text("No components are registered. Call gx.SetGallery with gxdev_gallery.Fixtures().")))
	}
	for _, f := range fixtures {
		if f.Missing {
			b.Add(El("section", Attrs{
				{Key: "class", Value: "fixture missing", Kind: AttrText},
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
			{Key: "data-fixture", Value: f.Component + "-" + f.Name, Kind: AttrText},
			{Key: "data-package", Value: f.Package, Kind: AttrText},
		},
			El("h2", nil, Text(f.Component+" - "+f.Name)),
			El("div", Attrs{{Key: "class", Value: "fixture-body", Kind: AttrText}}, body),
		))
	}
	return b.Node()
}

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
			El("body", nil, galleryNode()),
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
