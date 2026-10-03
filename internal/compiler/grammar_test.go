package compiler_test

import (
	"os"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

func TestREQ_AUT_20_GrammarSpec(t *testing.T) {
	data, err := os.ReadFile("../../docs/grammar.md")
	if err != nil {
		t.Fatalf("read grammar spec: %v", err)
	}
	doc := string(data)
	for _, want := range []string{
		"package clause",
		"import",
		"props",
		"signals",
		"if",
		"for",
		"switch",
		"case",
		"default",
		"fragment",
		"slot",
		"spread",
		"attribute",
		"expression",
		"comment",
		"void",
		"self-closing",
		"raw text",
		"GX1000",
		"GX1001",
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("grammar spec does not list %q", want)
		}
	}
}

func FuzzREQ_AUT_20_GrammarClosed(f *testing.F) {
	f.Add([]byte("<p>x</p>"))
	f.Add([]byte("<div class=\"a\">{p.Title}</div>"))
	f.Add([]byte("<ul>\n  for _, it := range p.Items {\n    <li>{it}</li>\n  }\n</ul>"))
	f.Add([]byte("<!-- x -->{/* y */}"))
	f.Fuzz(func(t *testing.T, src []byte) {
		file, diags := compiler.ParseFragment(src)
		if len(diags) > 0 {
			if file != nil {
				t.Fatalf("file and diagnostics returned together for %q", src)
			}
			return
		}
		if file == nil {
			t.Fatalf("nil file and no diagnostics for %q", src)
		}
		out := compiler.Format(file)
		again, diags := compiler.ParseFragment(out)
		if len(diags) > 0 {
			t.Fatalf("formatted output does not parse: %v\ninput: %q\noutput: %q", diags, src, out)
		}
		if got := compiler.Format(again); string(got) != string(out) {
			t.Fatalf("format is not idempotent\ninput: %q\nfirst: %q\nsecond: %q", src, out, got)
		}
	})
}
