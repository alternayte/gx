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
		`gx.Attr{Key: "data-bind", Value: gx.SignalName("cart.Cart", p.GxKey, "qty"), Kind: gx.AttrText}`,
		`gx.Client("data-show", "("+gx.SignalPath("cart.Cart", p.GxKey, "qty")+" > "+gx.JSON(1)+")", gx.ExprOp(">", gx.ExprSignal("cart.Cart", p.GxKey, "qty"), gx.ExprValue(1)))`,
		`gx.Client("data-text", gx.SignalPath("cart.Cart", p.GxKey, "qty"), gx.ExprSignal("cart.Cart", p.GxKey, "qty"))`,
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
			"\tif !strings.Contains(got, `data-bind`) {\n" +
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

// TestREQ_ACT_14_KeyedFragment checks that a fragment in a signal-bearing
// component takes the instance key and keys its id (REQ-ACT-14).
func TestREQ_ACT_14_KeyedFragment(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":       moduleWithGx(t),
		"cart/Cart.gx": "package cart\n\nprops {\n  Total int\n}\n\nsignals {\n  Qty int = 1\n}\n\n<div>\n  total := p.Total\n  <span #total(total int)>{total} <span text={$Qty}>1</span></span>\n</div>\n",
	})
	files := generateFiles(t, dir)
	src := string(files[filepath.Join(dir, "cart/Cart_gx.go")])
	for _, want := range []string{
		"func CartTotal(key gx.Key, total int) gx.Node {",
		`gx.Attr{Key: "id", Value: gx.FragmentID("cart", "total", key), Kind: gx.AttrText}`,
		`gx.SignalPath("cart.Cart", key, "qty")`,
	} {
		if !strings.Contains(src, want) {
			t.Errorf("Cart_gx.go lacks %q:\n%s", want, src)
		}
	}
}

// TestREQ_ACT_06_SignalRootElement covers where the first values of the
// signals go: on the first top-level HTML element. A component tag such as
// <gx.Head> before it does not take them, and a component with signals and
// no top-level HTML element is GX2015 (REQ-ACT-06).
func TestREQ_ACT_06_SignalRootElement(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod": moduleWithGx(t),
		"cart/Page.gx": "package cart\n\nsignals {\n  Open bool = false\n}\n\n" +
			"<gx.Head title=\"Cart\" />\n<h1>Cart</h1>\n<p show={$Open}>Open</p>\n",
		"cart/page_test.go": "package cart\n\nimport (\n\t\"strings\"\n\t\"testing\"\n\n\tgx \"github.com/alternayte/gx\"\n)\n\n" +
			"func TestPage(t *testing.T) {\n" +
			"\tgot := gx.String(Page(PageProps{}))\n" +
			"\tif !strings.Contains(got, `<h1 data-signals=\"{&#34;cart&#34;:{&#34;Page&#34;:{&#34;open&#34;:false}}}\" data-gx-instance=\"cart.Page\">Cart</h1>`) {\n" +
			"\t\tt.Fatalf(\"the first HTML element does not hold the signals: %s\", got)\n" +
			"\t}\n" +
			"}\n",
	})
	buildGenerated(t, dir, "test", "./...")

	dir = writeTree(t, map[string]string{
		"go.mod":        moduleWithGx(t),
		"cart/Card.gx":  "package cart\n\nprops {\n  Children gx.Node\n}\n\n<article>{p.Children}</article>\n",
		"cart/Panel.gx": "package cart\n\nsignals {\n  Open bool = false\n}\n\n<Card>\n  <p show={$Open}>Open</p>\n</Card>\n",
	})
	d := diagWith(t, checkDir(t, dir), compiler.CodeSignalRoot)
	if !strings.HasSuffix(d.File, "Panel.gx") || d.Line != 3 || !strings.Contains(d.Msg, "HTML element") || d.Fix == "" {
		t.Fatalf("GX2015 = %+v", d)
	}
}
