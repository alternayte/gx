package compiler_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

func generateWithDot(t *testing.T) (map[string][]byte, []compiler.Diagnostic) {
	t.Helper()
	return compiler.Generate(".")
}

func generateFiles(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	files, diags := compiler.Generate(dir)
	if len(diags) > 0 {
		t.Fatalf("Generate diagnostics: %v", diags)
	}
	return files
}

func TestREQ_AUT_02_GeneratedPropsAndDefaults(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":          moduleWithGx(t),
		"ui/card/Card.gx": "package card\n\nprops {\n  Title   string\n  Variant string = \"flat\"\n}\n\n<article><h3>{p.Title}</h3><span>{p.Variant}</span></article>\n",
		"ui/card/Page.gx": "package card\n\n<Card title=\"Hello\" />\n",
	})
	files := generateFiles(t, dir)

	card := string(files[filepath.Join(dir, "ui/card/Card_gx.go")])
	if card == "" {
		t.Fatal("Card_gx.go was not generated")
	}
	for _, want := range []string{"type CardProps struct {", "Title   string", "Variant string", "func Card(p CardProps) gx.Node {"} {
		if !strings.Contains(card, want) {
			t.Errorf("Card_gx.go lacks %q:\n%s", want, card)
		}
	}

	page := string(files[filepath.Join(dir, "ui/card/Page_gx.go")])
	if !strings.Contains(page, `Card(CardProps{Title: "Hello", Variant: "flat"})`) {
		t.Fatalf("Page_gx.go does not apply the default:\n%s", page)
	}
}

func TestREQ_AUT_02_CrossPackageDefaultQualified(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":            moduleWithGx(t),
		"ui/badge/Badge.gx": "package badge\n\nprops {\n  Label   string\n  Variant Variant = Default\n}\n\n<span>{p.Label}</span>\n",
		"ui/badge/types.go": "package badge\n\ntype Variant string\n\nconst Default Variant = \"flat\"\n",
		"pages/Home.gx":     "package pages\n\nimport \"app/ui/badge\"\n\n<badge.Badge label=\"New\" />\n",
	})
	files := generateFiles(t, dir)
	home := string(files[filepath.Join(dir, "pages/Home_gx.go")])
	if !strings.Contains(home, "badge.Default") {
		t.Fatalf("Home_gx.go does not qualify the default:\n%s", home)
	}
}

func TestREQ_AUT_02_GeneratedCodeRenders(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":          moduleWithGx(t),
		"ui/card/Card.gx": "package card\n\nprops {\n  Title   string\n  Variant string = \"flat\"\n}\n\n<article><h3>{p.Title}</h3><span>{p.Variant}</span></article>\n",
		"ui/card/List.gx": "package card\n\nprops {\n  Items []string\n}\n\n<ul>\n  for _, it := range p.Items {\n    <li>{it}</li>\n  }\n</ul>\n",
		"ui/card/Page.gx": "package card\n\n<Card title=\"Hello\" />\n<List items={[]string{\"a\", \"b\"}} />\n",
	})
	files := generateFiles(t, dir)
	for path, src := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, src, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mainSrc := `package main

import (
	"fmt"

	"app/ui/card"
	gx "github.com/alternayte/gx"
)

func main() {
	fmt.Print(gx.String(card.Page(card.PageProps{})))
}
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(mainSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", ".")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run: %v\n%s", err, out)
	}
	got := string(out)
	wantPrefix := `<article><h3>Hello</h3><span>flat</span></article>`
	if !strings.HasPrefix(got, wantPrefix) {
		t.Fatalf("output = %q, want prefix %q", got, wantPrefix)
	}
	for _, want := range []string{"<li>a</li>", "<li>b</li>"} {
		if !strings.Contains(got, want) {
			t.Fatalf("output = %q, want %q", got, want)
		}
	}
}
