package compiler_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

func TestREQ_AUT_09_Spread(t *testing.T) {
	// A spread on a component is GX2005.
	bad := writeTree(t, map[string]string{
		"go.mod":          moduleWithGx(t),
		"ui/card/Card.gx": "package card\n\nprops {\n  Attrs gx.Attrs\n}\n\n<div {...p.Attrs}></div>\n",
		"ui/card/Page.gx": "package card\n\nprops {\n  Extra gx.Attrs\n}\n\n<Card {...p.Extra} />\n",
	})
	diags := compiler.Check(bad)
	if !hasCode(diags, compiler.CodeSpread) {
		t.Fatalf("spread on a component: diagnostics = %v, want GX2005", diags)
	}

	// A spread on an HTML element takes a gx.Attrs value.
	dir := writeTree(t, map[string]string{
		"go.mod":          moduleWithGx(t),
		"ui/card/Box.gx":  "package card\n\nprops {\n  Attrs gx.Attrs\n}\n\n<div class=\"a\" {...p.Attrs}></div>\n",
		"ui/card/Page.gx": "package card\n\nimport \"github.com/alternayte/gx\"\n\nprops {\n}\n\n<Box attrs={gx.Attrs{gx.Attr{Key: \"data-x\", Value: \"1\"}}} />\n",
	})
	files, gdiags := compiler.Generate(dir)
	if len(gdiags) > 0 {
		t.Fatalf("Generate diagnostics: %v", gdiags)
	}
	box := string(files[filepath.Join(dir, "ui/card/Box_gx.go")])
	if !strings.Contains(box, "gx.JoinAttrs(") {
		t.Fatalf("Box_gx.go does not join the spread:\n%s", box)
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
	if got, want := strings.TrimSpace(string(out)), `<div class="a" data-x="1"></div>`; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}
