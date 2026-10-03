package compiler_test

import (
	"bytes"
	"go/format"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

func TestREQ_TLS_01_FmtGoParts(t *testing.T) {
	src := "package card\n\nimport (\n  \"b/x\"\n  \"a/y\"\n)\n\nprops {\n  Title string\n  Variant   string = \"flat\"\n}\n\n<p class={p.Item       +   \"x\"}>{len( p.Items )}</p>\n<ul>\n  for i:=0;i<len(p.Items);i++ {\n    <li>{i}</li>\n  }\n</ul>\n"
	out, diags := compiler.FormatSource("Card.gx", []byte(src))
	if len(diags) > 0 {
		t.Fatalf("diagnostics = %v", diags)
	}
	got := string(out)
	for _, want := range []string{`Title   string`, `Variant string = "flat"`, `{p.Item + "x"}`, `{len(p.Items)}`, `for i := 0; i < len(p.Items); i++ {`} {
		if !strings.Contains(got, want) {
			t.Fatalf("formatted output lacks %q:\n%s", want, got)
		}
	}
	if a, b := strings.Index(got, `import "a/y"`), strings.Index(got, `import "b/x"`); a < 0 || b < 0 || a > b {
		t.Fatalf("imports are not sorted:\n%s", got)
	}
	if out2, diags := compiler.FormatSource("Card.gx", out); len(diags) > 0 || !bytes.Equal(out2, out) {
		t.Fatalf("format is not idempotent or does not parse")
	}
}

func TestREQ_TLS_01_GeneratedGoIsGofmtClean(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":          moduleWithGx(t),
		"ui/card/Card.gx": "package card\n\nprops {\n  Title   string\n  Variant string = \"flat\"\n}\n\n<article><h3>{p.Title}</h3><span>{p.Variant}</span></article>\n",
		"ui/card/Page.gx": "package card\n\n<Card title=\"Hello\" />\n",
	})
	files := generateFiles(t, dir)
	for path, src := range files {
		formatted, err := format.Source(src)
		if err != nil {
			t.Fatalf("%s does not parse: %v", path, err)
		}
		if !bytes.Equal(formatted, src) {
			t.Fatalf("%s is not gofmt-clean:\n%s", path, src)
		}
	}
}
