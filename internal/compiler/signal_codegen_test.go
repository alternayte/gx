package compiler_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

const cartSignalsGx = `package cart

props {
  Start int
}

signals {
  Qty int = 1
}

<div>
  <input type="number" bind:value={$Qty} />
  <p show={$Qty > 1}>Over stock</p>
  <span text={$Qty}>1</span>
</div>
`

func TestREQ_ACT_05_SignalCodegen(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":       moduleWithGx(t),
		"cart/Cart.gx": cartSignalsGx,
	})
	files := generateFiles(t, dir)
	src := string(files[filepath.Join(dir, "cart/Cart_gx.go")])
	for _, want := range []string{
		"GxKey gx.Key",
		`gx.Attr{Key: "data-signals", Value: gx.SignalJSON("cart.Cart", p.GxKey, map[string]any{"qty": 1}), Kind: gx.AttrText}`,
		`gx.Attr{Key: "data-bind:value", Value: gx.SignalPath("cart.Cart", p.GxKey, "qty"), Kind: gx.AttrText}`,
		`gx.Attr{Key: "data-show", Value: "(" + gx.SignalPath("cart.Cart", p.GxKey, "qty") + " > " + gx.JSON(1) + ")", Kind: gx.AttrText}`,
		`gx.Attr{Key: "data-text", Value: gx.SignalPath("cart.Cart", p.GxKey, "qty"), Kind: gx.AttrText}`,
	} {
		if !strings.Contains(src, want) {
			t.Errorf("Cart_gx.go lacks %q:\n%s", want, src)
		}
	}
}

func TestREQ_ACT_06_SignalInstanceScope(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":       moduleWithGx(t),
		"cart/Cart.gx": cartSignalsGx,
		"cart/List.gx": "package cart\n\nprops {\n  IDs []int64\n}\n\nfor _, id := range p.IDs {\n  <Cart start={1} key={id} />\n}\n",
	})
	files := generateFiles(t, dir)
	src := string(files[filepath.Join(dir, "cart/List_gx.go")])
	if !strings.Contains(src, "GxKey: gx.InstanceKey(id)") {
		t.Fatalf("List_gx.go does not key the instance:\n%s", src)
	}
}

func TestREQ_ACT_06_MissingInstanceKey(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":       moduleWithGx(t),
		"cart/Cart.gx": cartSignalsGx,
		"cart/List.gx": "package cart\n\nprops {\n  IDs []int64\n}\n\nfor _, id := range p.IDs {\n  <Cart start={1} />\n}\n",
	})
	diags := checkDir(t, dir)
	d := diagWith(t, diags, compiler.CodeInstanceKey)
	if !strings.Contains(d.Msg, "<Cart>") {
		t.Fatalf("GX2012 message = %q", d.Msg)
	}

	ok := writeTree(t, map[string]string{
		"go.mod":       moduleWithGx(t),
		"cart/Cart.gx": cartSignalsGx,
		"cart/List.gx": "package cart\n\nprops {\n  IDs []int64\n}\n\nfor _, id := range p.IDs {\n  <div key={id}><Cart start={1} /></div>\n}\n",
	})
	for _, d := range checkDir(t, ok) {
		if d.Code == compiler.CodeInstanceKey {
			t.Fatalf("keyed ancestor still reports GX2012: %v", d)
		}
	}
}

func TestREQ_ACT_06_SignalNeedsDefault(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":       moduleWithGx(t),
		"cart/Cart.gx": "package cart\n\nsignals {\n  Qty int\n}\n\n<p>x</p>\n",
	})
	diags := checkDir(t, dir)
	d := diagWith(t, diags, compiler.CodeSignalDefault)
	if !strings.Contains(d.Msg, `"Qty"`) {
		t.Fatalf("GX2014 message = %q", d.Msg)
	}
}

func TestREQ_ACT_06_SignalPathMatchesSetSignals(t *testing.T) {
	// An action patch and the page render must agree on the scope: both
	// use ScopeString with the same base and key (REQ-ACT-05,
	// REQ-ACT-06). The render half is checked here; the patch half is
	// checked by TestREQ_ACT_01_SetSignalsAnswer.
	dir := writeTree(t, map[string]string{
		"go.mod":       moduleWithGx(t),
		"cart/Cart.gx": cartSignalsGx,
		"cart/render_test.go": "package cart\n\nimport (\n\t\"strings\"\n\t\"testing\"\n\n\t\"github.com/alternayte/gx\"\n)\n\n" +
			"func TestRender(t *testing.T) {\n" +
			"\tgot := gx.String(Cart(CartProps{GxKey: \"42\"}))\n" +
			"\tif !strings.Contains(got, `data-gx-instance=\"cart.Cart.42\"`) {\n" +
			"\t\tt.Fatalf(\"render = %s\", got)\n" +
			"\t}\n" +
			"\tif !strings.Contains(got, `qty`) {\n" +
			"\t\tt.Fatalf(\"signals missing: %s\", got)\n" +
			"\t}\n" +
			"\tif !strings.Contains(got, `data-bind:value`) {\n" +
			"\t\tt.Fatalf(\"bind missing: %s\", got)\n" +
			"\t}\n" +
			"}\n",
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
	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go test: %v\n%s", err, out)
	}
}
