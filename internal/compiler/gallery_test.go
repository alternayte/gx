package compiler_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

// TestREQ_AI_03_Fixtures covers the generated gallery registry: one entry
// per fixture and a Missing entry for a component without fixtures
// (REQ-AI-03).
func TestREQ_AI_03_Fixtures(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":              moduleWithGx(t),
		"ui/Card.gx":          "package ui\n\nprops {\n  Title string\n}\n\n<article>{p.Title}</article>\n",
		"ui/Card.fixtures.go": "package ui\n\nimport \"github.com/alternayte/gx\"\n\nvar Fixtures = gx.Fixtures[CardProps]{\n\t\"Default\": {Title: \"Hello\"},\n\t\"Long\":    {Title: \"A longer title\"},\n}\n",
		"ui/Button.gx":        "package ui\n\n<button>x</button>\n",
	})
	files, diags := compiler.Generate(dir)
	if len(diags) > 0 {
		t.Fatalf("generate: %v", diags)
	}
	data, ok := files[filepath.Join(dir, "gxdev_gallery", "gallery_gx.go")]
	if !ok {
		t.Fatalf("no gallery file in %v", files)
	}
	got := string(data)
	for _, want := range []string{
		"//go:build gxdev",
		"package gxdev_gallery",
		`{Component: "Card", Package: "app/ui", Name: "Default", Node: func() gx.Node { return ui.Card(ui.Fixtures["Default"]) }}`,
		`{Component: "Card", Package: "app/ui", Name: "Long", Node: func() gx.Node { return ui.Card(ui.Fixtures["Long"]) }}`,
		`{Component: "Button", Package: "app/ui", Missing: true}`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("gallery file lacks %q:\n%s", want, got)
		}
	}
}
