package compiler_test

import (
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

// TestREQ_CNT_03_ContentComponents covers the content component checks:
// GX8002 for a component outside the collection's Components list, GX2003
// for an unknown prop, and no findings inside code or comments
// (REQ-CNT-03).
func TestREQ_CNT_03_ContentComponents(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":                moduleWithGx(t),
		"docs/Aside.gx":         "package docs\n\nprops {\n  Kind string\n}\n\n<aside>{p.Kind}</aside>\n",
		"docs/content.go":       "package docs\n\nimport \"github.com/alternayte/gx\"\n\ntype DocMeta struct {\n\tTitle string `yaml:\"title\"`\n}\n\nvar Docs = gx.Collection[DocMeta](\"content/docs\").Components(Aside)\n",
		"content/docs/start.md": "---\ntitle: Start\n---\n\nintro\n\n<docs.Aside kind=\"tip\">Good</docs.Aside>\n<docs.Widget nope=\"1\" />\n<docs.Aside wrong=\"x\" />\n\n```\n<docs.NotAComponent />\n```\n\n`<docs.Inline />`\n",
	})
	diags := compiler.Check(dir)
	got8002, got2003 := 0, 0
	for _, d := range diags {
		switch d.Code {
		case compiler.CodeContentComponent:
			got8002++
			if !strings.Contains(d.Msg, "Widget") {
				t.Errorf("GX8002 message = %q", d.Msg)
			}
			if !strings.HasSuffix(d.File, "start.md") || d.Line != 8 {
				t.Errorf("GX8002 position = %s:%d", d.File, d.Line)
			}
		case compiler.CodeUnknownAttr:
			got2003++
			if !strings.Contains(d.Msg, "wrong") && !strings.Contains(d.Msg, "nope") {
				t.Errorf("GX2003 message = %q", d.Msg)
			}
		}
	}
	if got8002 != 1 {
		t.Fatalf("GX8002 count = %d, want 1: %v", got8002, diags)
	}
	if got2003 != 1 {
		t.Fatalf("GX2003 count = %d, want 1: %v", got2003, diags)
	}
}
