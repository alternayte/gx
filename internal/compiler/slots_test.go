package compiler_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

const slotsCart = `package cart

props {
  Items    []string
  Row      gx.Slot[string]
  Header   gx.Node = nil
  Children gx.Node
}

<section>
  if p.Header != nil {
    <header>{p.Header}</header>
  }
  <ul>
    for _, it := range p.Items {
      <li>{p.Row(it)}</li>
    }
  </ul>
  {p.Children}
</section>
`

func TestREQ_AUT_11_Slots(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":          moduleWithGx(t),
		"ui/cart/Cart.gx": slotsCart,
		"ui/cart/Page.gx": "package cart\n\nprops {\n  Items []string\n}\n\n<Cart items={p.Items}>\n  <:header>Your cart</:header>\n  <:row let={it}>{it}</:row>\n  <p>done</p>\n</Cart>\n",
	})
	files := generateFiles(t, dir)

	page := string(files[filepath.Join(dir, "ui/cart/Page_gx.go")])
	for _, want := range []string{"Row: func(it string) gx.Node {", "Header:", "Children:"} {
		if !strings.Contains(page, want) {
			t.Fatalf("Page_gx.go lacks %q:\n%s", want, page)
		}
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

	"app/ui/cart"
	gx "github.com/alternayte/gx"
)

func main() {
	fmt.Print(gx.String(cart.Page(cart.PageProps{Items: []string{"a", "b"}})))
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
	for _, want := range []string{"<header>Your cart</header>", "<li>a</li>", "<li>b</li>", "<p>done</p>"} {
		if !strings.Contains(got, want) {
			t.Fatalf("output = %q, want %q", got, want)
		}
	}
}

func TestREQ_AUT_11_DuplicateSlot(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":          moduleWithGx(t),
		"ui/cart/Cart.gx": slotsCart,
		"ui/cart/Page.gx": "package cart\n\nprops {\n  Items []string\n}\n\n<Cart items={p.Items}>\n  <:row let={a}>{a}</:row>\n  <:row let={b}>{b}</:row>\n</Cart>\n",
	})
	diags := compiler.Check(dir)
	if !hasCode(diags, compiler.CodeDuplicateSlot) {
		t.Fatalf("duplicate slot: diagnostics = %v, want GX2006", diags)
	}
}
