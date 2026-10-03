package compiler_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

func TestREQ_AUT_10_ClassOrder(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":           moduleWithGx(t),
		"ui/card/Badge.gx": "package card\n\nprops {\n  On   bool\n  Also bool\n  User string\n}\n\n<div class:first={p.On} class=\"mid\" class:last={p.Also}></div>\n<span class={p.User}></span>\n",
		"ui/card/Page.gx":  "package card\n\n<Badge on={true} also={false} user=\"u\" />\n",
	})
	files := generateFiles(t, dir)

	badge := string(files[filepath.Join(dir, "ui/card/Badge_gx.go")])
	wantExpr := `gx.Classes(gx.When("first", p.On), "mid", gx.When("last", p.Also))`
	if !strings.Contains(badge, wantExpr) {
		t.Fatalf("class parts do not join in source order:\n%s", badge)
	}
	if !strings.Contains(badge, `Value: p.User`) {
		t.Fatalf("class expression is not the attribute value:\n%s", badge)
	}

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
	if !strings.Contains(got, `class="first mid"`) {
		t.Fatalf("output = %q, want class \"first mid\"", got)
	}
	if !strings.Contains(got, `class="u"`) {
		t.Fatalf("output = %q, want class \"u\"", got)
	}
}

func TestREQ_AUT_10_ClassNeedsBool(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":           moduleWithGx(t),
		"ui/card/Badge.gx": "package card\n\nprops {\n  Name string\n}\n\n<div class:first={p.Name}></div>\n",
	})
	diags := compiler.Check(dir)
	if !hasCode(diags, compiler.CodeType) {
		t.Fatalf("class directive with a string: diagnostics = %v, want GX2000", diags)
	}
}
