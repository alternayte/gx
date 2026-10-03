package compiler_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func checkDir(t *testing.T, dir string) []compiler.Diagnostic {
	t.Helper()
	return compiler.Check(dir)
}

func diagWith(t *testing.T, diags []compiler.Diagnostic, code string) compiler.Diagnostic {
	t.Helper()
	for _, d := range diags {
		if d.Code == code {
			return d
		}
	}
	t.Fatalf("no %s diagnostic in %v", code, diags)
	return compiler.Diagnostic{}
}

const cardGx = `package card

props {
  Title   string
  Variant string = "flat"
}

<article>{p.Title}</article>
`

func TestREQ_AUT_02_MissingRequiredProp(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":          moduleWithGx(t),
		"ui/card/Card.gx": cardGx,
		"ui/card/Page.gx": "package card\n\n<Card />\n",
	})
	diags := compiler.Check(dir)
	d := diagWith(t, diags, compiler.CodeRequiredProp)
	if !strings.Contains(d.Msg, `"Title"`) || !strings.Contains(d.Msg, "<Card>") {
		t.Fatalf("GX2001 message = %q", d.Msg)
	}
	if !strings.HasSuffix(d.File, "Page.gx") || d.Line != 3 {
		t.Fatalf("GX2001 position = %s:%d, want Page.gx:3", d.File, d.Line)
	}
	if len(diags) != 1 {
		t.Fatalf("diagnostics = %v, want only GX2001", diags)
	}

	ok := writeTree(t, map[string]string{
		"go.mod":          moduleWithGx(t),
		"ui/card/Card.gx": cardGx,
		"ui/card/Page.gx": "package card\n\n<Card title=\"x\" />\n",
	})
	if diags := compiler.Check(ok); len(diags) != 0 {
		t.Fatalf("provided prop: unexpected diagnostics %v", diags)
	}
}

func TestREQ_AUT_02_OptionalPropIsNotRequired(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":          moduleWithGx(t),
		"ui/card/Card.gx": cardGx,
		"ui/card/Page.gx": "package card\n\n<Card title=\"x\" variant=\"wide\" />\n",
	})
	if diags := compiler.Check(dir); len(diags) != 0 {
		t.Fatalf("optional prop with a value: unexpected diagnostics %v", diags)
	}
}

func TestREQ_AUT_06_UnknownComponentSuggest(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":              moduleWithGx(t),
		"ui/button/Button.gx": "package button\n\n<button>Go</button>\n",
		"ui/button/Page.gx":   "package button\n\n<Buton />\n<div></div>\n<my-widget></my-widget>\n",
	})
	diags := compiler.Check(dir)
	d := diagWith(t, diags, compiler.CodeUnknownComponent)
	if !strings.Contains(d.Msg, `did you mean "Button"?`) {
		t.Fatalf("GX2002 message = %q, want a suggestion", d.Msg)
	}
	if len(diags) != 1 {
		t.Fatalf("diagnostics = %v, want only GX2002", diags)
	}
}

func TestREQ_AUT_06_CrossPackageComponent(t *testing.T) {
	files := map[string]string{
		"go.mod":          moduleWithGx(t),
		"ui/card/Card.gx": "package card\n\nprops {\n  Title string\n}\n\n<article>{p.Title}</article>\n",
		"pages/Home.gx":   "package pages\n\nimport \"app/ui/card\"\n\n<card.Card title=\"x\" />\n",
	}
	dir := writeTree(t, files)
	if diags := compiler.Check(dir); len(diags) != 0 {
		t.Fatalf("cross-package call: unexpected diagnostics %v", diags)
	}

	files["pages/Home.gx"] = "package pages\n\nimport \"app/ui/card\"\n\n<card.Card />\n"
	dir = writeTree(t, files)
	d := diagWith(t, compiler.Check(dir), compiler.CodeRequiredProp)
	if !strings.Contains(d.Msg, "<card.Card>") {
		t.Fatalf("GX2001 message = %q", d.Msg)
	}
}

func TestREQ_AUT_07_AttributeMapping(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":           moduleWithGx(t),
		"ui/card/Card.gx":  "package card\n\nprops {\n  Title   string\n  Variant Variant = \"flat\"\n}\n\n<article>{p.Title}</article>\n",
		"ui/card/types.go": "package card\n\ntype Variant string\n",
		"ui/card/Page.gx":  "package card\n\n<Card title=\"x\" variant=\"raised\" />\n<Card title=\"x\" foo=\"1\" />\n",
	})
	diags := compiler.Check(dir)
	d := diagWith(t, diags, compiler.CodeStaticStringProp)
	if !strings.Contains(d.Msg, `"variant"`) || !strings.Contains(d.Msg, `"Variant"`) {
		t.Fatalf("GX2004 message = %q", d.Msg)
	}
	d = diagWith(t, diags, compiler.CodeUnknownAttr)
	if !strings.Contains(d.Msg, `"foo"`) || !strings.Contains(d.Msg, "<Card>") {
		t.Fatalf("GX2003 message = %q", d.Msg)
	}
	if len(diags) != 2 {
		t.Fatalf("diagnostics = %v, want GX2004 and GX2003", diags)
	}
}
