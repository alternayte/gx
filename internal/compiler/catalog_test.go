package compiler_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

func TestREQ_AUT_19_DiagnosticCatalog(t *testing.T) {
	seen := map[string]bool{}
	var got []string
	for _, info := range compiler.Catalog {
		if info.Code == "" || info.Title == "" {
			t.Fatalf("incomplete catalog entry %+v", info)
		}
		if seen[info.Code] {
			t.Errorf("duplicate catalog entry %s", info.Code)
		}
		seen[info.Code] = true
		got = append(got, info.Code+" | "+info.Title)
		page := filepath.Join("..", "..", "docs", "errors", info.Code+".md")
		if _, err := os.Stat(page); err != nil {
			t.Errorf("no docs page for %s: %v", info.Code, err)
		}
	}
	want := []string{
		"GX1000 | parse error",
		"GX1001 | component file name is not an exported identifier",
		"GX1002 | generated code is missing or stale",
		"GX2000 | type error in an expression",
		"GX2001 | missing required prop",
		"GX2002 | unknown component",
		"GX2003 | unknown attribute or slot",
		"GX2004 | static value for a typed prop",
		"GX2005 | attribute spread on a component",
		"GX2006 | duplicate slot",
		"GX2007 | dynamic event attribute",
		"GX2008 | fragment free variable",
		"GX2009 | loop needs a key",
		"GX2010 | signal in a server expression",
		"GX2011 | dynamic URL attribute",
		"GX2013 | value cannot render as text",
		"GX3000 | route field type cannot bind",
		"GX3001 | pattern variable has no field",
		"GX3002 | path field has no pattern variable",
		"GX3003 | route value is not held by any gx.Collect",
		"GX3004 | duplicate route pattern",
		"GX3005 | route package contents",
		"GX7001 | conversion to gx.SafeHTML needs //gx:trusted",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("catalog changed; update the snapshot deliberately:\ngot:\n%v\nwant:\n%v", got, want)
	}
	if doc := (compiler.Diagnostic{Code: compiler.CodeType}).Doc(); doc != "/errors/GX2000" {
		t.Fatalf("Doc = %q", doc)
	}
}
