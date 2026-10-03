package compiler_test

import (
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

func hasCode(diags []compiler.Diagnostic, code string) bool {
	for _, d := range diags {
		if d.Code == code {
			return true
		}
	}
	return false
}

func TestREQ_AUT_01_ComponentName(t *testing.T) {
	src := []byte("package card\n\nprops {\n  Title string\n}\n\n<article>{p.Title}</article>\n")

	f, diags := compiler.ParseFile("ui/card/Card.gx", src)
	if len(diags) != 0 {
		t.Fatalf("valid file: unexpected diagnostics: %v", diags)
	}
	if f.Package != "card" {
		t.Errorf("package = %q, want card", f.Package)
	}
	if !f.HasProps || len(f.Props) != 1 || f.Props[0].Name != "Title" {
		t.Errorf("props = %+v, want one field Title", f.Props)
	}

	_, diags = compiler.ParseFile("ui/card/card.gx", src)
	if !hasCode(diags, compiler.CodeFileName) {
		t.Errorf("lowercase file name: diagnostics = %v, want GX1001", diags)
	}

	_, diags = compiler.ParseFile("ui/card/Card-2.gx", src)
	if !hasCode(diags, compiler.CodeFileName) {
		t.Errorf("non-identifier file name: diagnostics = %v, want GX1001", diags)
	}

	f, diags = compiler.ParseFile("ui/card/Card2.gx", src)
	if len(diags) != 0 || f.Package != "card" {
		t.Errorf("Card2.gx: diagnostics = %v, want none", diags)
	}

	_, diags = compiler.ParseFile("Card.gx", []byte("<p>no package</p>\n"))
	if !hasCode(diags, compiler.CodeParse) {
		t.Errorf("missing package clause: diagnostics = %v, want GX1000", diags)
	}

	f, diags = compiler.ParseFile("", src)
	if len(diags) != 0 {
		t.Errorf("empty file name skips the name check: diagnostics = %v", diags)
	}
	if f == nil {
		t.Fatal("empty file name returned a nil file")
	}
}
