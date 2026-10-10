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
		page := filepath.Join("..", "..", "docs", "content", "errors", info.Code+".md")
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
		"GX2012 | signal instance needs a key",
		"GX2014 | signal needs an initial value",
		"GX2015 | component with signals has no top-level HTML element",
		"GX2011 | dynamic URL attribute",
		"GX2013 | value cannot render as text",
		"GX3000 | route field type cannot bind",
		"GX3001 | pattern variable has no field",
		"GX3002 | path field has no pattern variable",
		"GX3003 | route value is not held by any gx.Collect",
		"GX3004 | duplicate route pattern",
		"GX3005 | route package contents",
		"GX4001 | no action is registered for a route type",
		"GX4002 | more than one action is registered for a route type",
		"GX4003 | no signal is declared for a signal-bound field",
		"GX4004 | signal type does not match the action field",
		"GX4005 | call is not allowed in a client expression",
		"GX4006 | signal or client expression under an adapter with no signals",
		"GX4007 | value or operator is not allowed in a client expression",
		"GX4008 | signal-bound fields need rules or gx.Unchecked",
		"GX4009 | action method cannot be invoked from the client",
		"GX4010 | unknown event modifier or special event",
		"GX4011 | tool has no description or an input with no JSON form",
		"GX4012 | c.Update takes a component with no fragment",
		"GX5001 | gx.Enum misses a constant of its type",
		"GX5002 | duplicate view-transition-name in one template",
		"GX5003 | class string is built at runtime",
		"GX6001 | island has no props struct",
		"GX6002 | island prop type has no TypeScript mapping",
		"GX6003 | widget tag is missing, not valid or used two times",
		"GX6004 | unknown island load strategy",
		"GX6005 | TypeScript error in an island",
		"GX6006 | widget input field cannot be an attribute",
		"GX6007 | gx.Head in a widget",
		"GX6008 | widget route is in a group with no origins",
		"GX6009 | origin with cookies is not exact",
		"GX7001 | conversion to gx.SafeHTML needs //gx:trusted",
		"GX7002 | gx.Secret cannot cross to the client",
		"GX8001 | frontmatter is malformed or unknown",
		"GX8002 | component is not declared in this collection",
		"GX8003 | content link or anchor is broken",
		"GX8004 | code file or line range is missing",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("catalog changed; update the snapshot deliberately:\ngot:\n%v\nwant:\n%v", got, want)
	}
	if doc := (compiler.Diagnostic{Code: compiler.CodeType}).Doc(); doc != "/errors/GX2000" {
		t.Fatalf("Doc = %q", doc)
	}
}
