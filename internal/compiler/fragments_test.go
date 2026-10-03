package compiler_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

func TestREQ_AUT_13_FragmentFunction(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":          moduleWithGx(t),
		"ui/cart/Cart.gx": "package cart\n\nprops {\n  Items []string\n}\n\n<section>\n  total := len(p.Items)\n  <span #total(total int)>{total} items</span>\n</section>\n",
	})
	files := generateFiles(t, dir)
	card := string(files[filepath.Join(dir, "ui/cart/Cart_gx.go")])
	for _, want := range []string{"func CartTotal(total int) gx.Node {", `Value: "cart-total"`} {
		if !strings.Contains(card, want) {
			t.Fatalf("Cart_gx.go lacks %q:\n%s", want, card)
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
	"strings"

	"app/ui/cart"
	gx "github.com/alternayte/gx"
)

func main() {
	full := gx.String(cart.Cart(cart.CartProps{Items: []string{"a", "b"}}))
	frag := gx.String(cart.CartTotal(2))
	if !strings.Contains(full, frag) {
		fmt.Print("MISMATCH full=" + full + " frag=" + frag)
		return
	}
	fmt.Print(frag)
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
	if got, want := string(out), `<span id="cart-total">2 items</span>`; got != want {
		t.Fatalf("fragment render = %q, want %q", got, want)
	}
}

func TestREQ_AUT_13_FragmentUsesProps(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":          moduleWithGx(t),
		"ui/cart/Cart.gx": "package cart\n\nprops {\n  Items []string\n}\n\n<ul #items>{len(p.Items)}</ul>\n",
	})
	files := generateFiles(t, dir)
	card := string(files[filepath.Join(dir, "ui/cart/Cart_gx.go")])
	if !strings.Contains(card, "func CartItems(p CartProps) gx.Node {") {
		t.Fatalf("a fragment that uses p does not take the props:\n%s", card)
	}
}

func TestREQ_AUT_13_FreeVariable(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":          moduleWithGx(t),
		"ui/cart/Cart.gx": "package cart\n\nprops {\n  Items []string\n}\n\n<section>\n  total := len(p.Items)\n  <span #total(n int)>{total}</span>\n</section>\n",
	})
	diags := checkDir(t, dir)
	d := diagWith(t, diags, compiler.CodeFragment)
	if !strings.Contains(d.Msg, "total") {
		t.Fatalf("GX2008 message = %q", d.Msg)
	}
	if !strings.Contains(d.Fix, "total") {
		t.Fatalf("GX2008 fix = %q, want a fix that names total", d.Fix)
	}
}
