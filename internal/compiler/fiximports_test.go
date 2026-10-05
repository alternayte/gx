package compiler

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestNFR_05_GenerateNeedsNoPackageSearch pins the cold build budget: the
// generator writes the imports of the example app without a search of the
// module cache, and still removes an import the code does not use.
func TestNFR_05_GenerateNeedsNoPackageSearch(t *testing.T) {
	src := "package x\n\nimport (\n\t\"strings\"\n\t\"example.com/app/ui/input\"\n\tv2 \"example.com/lib/v2\"\n\t\"example.com/other/v3\"\n)\n\nvar _ = strings.ToUpper(v2.Name + other.Name)\n"
	before := packageSearches.Load()
	out, err := fixImports("x_gx.go", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if strings.Contains(got, "ui/input") {
		t.Errorf("the unused import stays:\n%s", got)
	}
	for _, want := range []string{`"strings"`, `v2 "example.com/lib/v2"`, `"example.com/other/v3"`} {
		if !strings.Contains(got, want) {
			t.Errorf("the used import %s is gone:\n%s", want, got)
		}
	}
	if _, diags := NewSession().Generate(filepath.Join("..", "..", "examples", "shop")); len(diags) > 0 {
		t.Fatalf("generate: %v", diags)
	}
	if n := packageSearches.Load() - before; n != 0 {
		t.Errorf("NFR-05: %d generated files needed a package search, want 0", n)
	}
}
