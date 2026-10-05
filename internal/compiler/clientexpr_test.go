package compiler_test

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

func cartWithSignals(body string) string {
	return "package cart\n\nimport \"github.com/alternayte/gx/gxc\"\n\nprops {\n  Total int\n  Name  string\n  Open  bool\n  Items []int\n  A     Point\n  B     Point\n}\n\nsignals {\n  Qty  int  = 1\n  Open2 bool  = false\n  Text string = \"\"\n}\n\n" + body + "\n"
}

// TestREQ_ACT_07_TranspileGolden checks every allowed client form.
func TestREQ_ACT_07_TranspileGolden(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			"show value with inlined server value",
			"<p show={$Qty > p.Total}>x</p>",
			`gx.Attr{Key: "data-show", Value: "(" + gx.SignalPath("cart.Cart", p.GxKey, "qty") + " > " + gx.JSON(p.Total) + ")", Kind: gx.AttrText}`,
		},
		{
			"text value",
			"<span text={$Qty}>x</span>",
			`gx.Attr{Key: "data-text", Value: gx.SignalPath("cart.Cart", p.GxKey, "qty"), Kind: gx.AttrText}`,
		},
		{
			"bind",
			"<input bind:value={$Qty} />",
			`gx.Attr{Key: "data-bind", Value: gx.SignalName("cart.Cart", p.GxKey, "qty"), Kind: gx.AttrText}`,
		},
		{
			"class",
			"<p class:ring={$Open2}>x</p>",
			`gx.Attr{Key: "data-class:ring", Value: gx.SignalPath("cart.Cart", p.GxKey, "open2"), Kind: gx.AttrText}`,
		},
		{
			"attr",
			`<p attr:aria-expanded={$Open2}>x</p>`,
			`gx.Attr{Key: "data-attr:aria-expanded", Value: gx.SignalPath("cart.Cart", p.GxKey, "open2"), Kind: gx.AttrText}`,
		},
		{
			"on assignment",
			"<button on:click={$Open2 = !$Open2}>x</button>",
			"gx.Attr{Key: \"data-on:click\", Value: gx.SignalPath(\"cart.Cart\", p.GxKey, \"open2\") + \" = \" + \"!\" + gx.SignalPath(\"cart.Cart\", p.GxKey, \"open2\"), Kind: gx.AttrText}",
		},
		{
			"on increment",
			"<button on:click={$Qty++}>x</button>",
			"gx.Attr{Key: \"data-on:click\", Value: gx.SignalPath(\"cart.Cart\", p.GxKey, \"qty\") + \"++\", Kind: gx.AttrText}",
		},
		{
			"gxc helper on a signal",
			"<p show={gxc.Len($Text) > $Qty}>x</p>",
			`"__gx.len(" + gx.SignalPath("cart.Cart", p.GxKey, "text") + ")" + " > " + gx.SignalPath("cart.Cart", p.GxKey, "qty")`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := writeTree(t, map[string]string{
				"go.mod":        moduleWithGx(t),
				"cart/types.go": "package cart\n\ntype Point struct{ X int }\n",
				"cart/Cart.gx":  cartWithSignals(c.body),
			})
			files := generateFiles(t, dir)
			src := string(files[filepath.Join(dir, "cart/Cart_gx.go")])
			if !strings.Contains(src, c.want) {
				t.Fatalf("generated code lacks %q:\n%s", c.want, src)
			}
		})
	}
}

// TestREQ_ACT_07_SignalStatements checks a multi-statement handler.
func TestREQ_ACT_07_SignalStatements(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":        moduleWithGx(t),
		"cart/types.go": "package cart\n\ntype Point struct{ X int }\n",
		"cart/Cart.gx":  cartWithSignals("<button on:click={$Qty = $Qty + 1; $Open2 = true; $Qty--}>x</button>"),
	})
	files := generateFiles(t, dir)
	src := string(files[filepath.Join(dir, "cart/Cart_gx.go")])
	for _, want := range []string{`+ "; " +`, `+ " = " +`, `+ "--"`} {
		if !strings.Contains(src, want) {
			t.Fatalf("statement codegen lacks %q:\n%s", want, src)
		}
	}
}

// TestREQ_ACT_07_BadCall checks GX4005 names the call.
func TestREQ_ACT_07_BadCall(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":        moduleWithGx(t),
		"cart/types.go": "package cart\n\ntype Point struct{ X int }\n",
		"cart/Cart.gx":  cartWithSignals("<p show={len(p.Name) > $Qty}>x</p>"),
	})
	diags := checkDir(t, dir)
	d := diagWith(t, diags, compiler.CodeClientCall)
	if !strings.Contains(d.Msg, "gxc") {
		t.Fatalf("GX4005 message = %q", d.Msg)
	}
}

// TestREQ_ACT_07_BindTargetsSignal checks that bind: takes one signal.
func TestREQ_ACT_07_BindTargetsSignal(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":        moduleWithGx(t),
		"cart/types.go": "package cart\n\ntype Point struct{ X int }\n",
		"cart/Cart.gx":  cartWithSignals("<input bind:value={$Qty + 1} />"),
	})
	diags := checkDir(t, dir)
	d := diagWith(t, diags, compiler.CodeClientType)
	if !strings.Contains(d.Msg, "bind") {
		t.Fatalf("GX4007 message = %q", d.Msg)
	}
}

// TestREQ_ACT_07_UnknownHelper checks GX4005 for a gxc helper with no JS
// equivalent.
func TestREQ_ACT_07_UnknownHelper(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":        moduleWithGx(t),
		"cart/types.go": "package cart\n\ntype Point struct{ X int }\n",
		"cart/Cart.gx":  cartWithSignals("<p show={gxc.Split(p.Name) > $Qty}>x</p>"),
	})
	diags := checkDir(t, dir)
	d := diagWith(t, diags, compiler.CodeClientCall)
	if !strings.Contains(d.Msg, "gxc.Split") {
		t.Fatalf("GX4005 message = %q", d.Msg)
	}
}

// TestREQ_ACT_07_SignalRef checks a child shares a parent signal through a
// gx.SignalRef prop.
func TestREQ_ACT_07_SignalRef(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":              moduleWithGx(t),
		"ui/dialog/Dialog.gx": "package dialog\n\nimport \"github.com/alternayte/gx\"\n\nprops {\n  Open gx.SignalRef[bool]\n}\n\n<div show={p.Open}></div>\n",
		"ui/page/Page.gx":     "package page\n\nimport \"app/ui/dialog\"\n\nsignals {\n  Open bool = false\n}\n\n<div>\n  <dialog.Dialog open={$Open} />\n</div>\n",
	})
	files := generateFiles(t, dir)
	page := string(files[filepath.Join(dir, "ui/page/Page_gx.go")])
	if !strings.Contains(page, `gx.Ref[bool](gx.SignalRefPath("page.Page", p.GxKey, "open"))`) {
		t.Fatalf("Page_gx.go does not pass the signal ref:\n%s", page)
	}
	dialog := string(files[filepath.Join(dir, "ui/dialog/Dialog_gx.go")])
	if !strings.Contains(dialog, `"$" + gx.RefPath(p.Open)`) {
		t.Fatalf("Dialog_gx.go does not read the signal ref:\n%s", dialog)
	}
}

// TestREQ_ACT_08_EventMapping checks every event modifier and special event
// on the wire.
func TestREQ_ACT_08_EventMapping(t *testing.T) {
	cases := []struct {
		attr string
		want string
	}{
		{"on:click.prevent={$Qty++}", "data-on:click__prevent"},
		{"on:click.stop.once={$Qty++}", "data-on:click__stop__once"},
		{"on:click.outside={$Qty++}", "data-on:click__outside"},
		{"on:keydown.window={$Qty++}", "data-on:keydown__window"},
		{"on:input.debounce(300ms)={$Qty++}", "data-on:input__debounce.300ms"},
		{"on:scroll.throttle(1s)={$Qty++}", "data-on:scroll__throttle.1s"},
		{"on:load={$Qty++}", "data-on-init"},
		{"on:visible={$Qty++}", "data-on-intersect"},
		{"on:interval(5s)={$Qty++}", "data-on-interval__duration.5s"},
	}
	for _, c := range cases {
		t.Run(c.attr, func(t *testing.T) {
			dir := writeTree(t, map[string]string{
				"go.mod":        moduleWithGx(t),
				"cart/types.go": "package cart\n\ntype Point struct{ X int }\n",
				"cart/Cart.gx":  cartWithSignals("<div " + c.attr + ">x</div>"),
			})
			files := generateFiles(t, dir)
			src := string(files[filepath.Join(dir, "cart/Cart_gx.go")])
			if !strings.Contains(src, strconv.Quote(c.want)) {
				t.Fatalf("codegen lacks %q:\n%s", c.want, src)
			}
		})
	}
}

// TestREQ_ACT_08_UnknownModifier checks GX4010.
func TestREQ_ACT_08_UnknownModifier(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":        moduleWithGx(t),
		"cart/types.go": "package cart\n\ntype Point struct{ X int }\n",
		"cart/Cart.gx":  cartWithSignals("<div on:click.hover={$Qty++}>x</div>"),
	})
	diags := checkDir(t, dir)
	d := diagWith(t, diags, compiler.CodeEventMod)
	if !strings.Contains(d.Msg, "hover") {
		t.Fatalf("GX4010 message = %q", d.Msg)
	}
}

// TestREQ_ACT_13_IntDivision checks that int division uses Math.trunc.
func TestREQ_ACT_13_IntDivision(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":        moduleWithGx(t),
		"cart/types.go": "package cart\n\ntype Point struct{ X int }\n",
		"cart/Cart.gx":  cartWithSignals("<p show={$Qty / 2 > 0}>x</p>"),
	})
	files := generateFiles(t, dir)
	src := string(files[filepath.Join(dir, "cart/Cart_gx.go")])
	if !strings.Contains(src, `Math.trunc`) {
		t.Fatalf("int division does not truncate:\n%s", src)
	}
}

// TestREQ_ACT_13_BadCompare checks GX4007 for equality on a struct.
func TestREQ_ACT_13_BadCompare(t *testing.T) {
	body := "signals {\n  P Point = Point{}\n  Q Point = Point{}\n}\n\n<p show={$P == $Q}>x</p>"
	dir := writeTree(t, map[string]string{
		"go.mod":        moduleWithGx(t),
		"cart/types.go": "package cart\n\ntype Point struct{ X int }\n",
		"cart/Cart.gx":  "package cart\n\n" + body + "\n",
	})
	diags := checkDir(t, dir)
	d := diagWith(t, diags, compiler.CodeClientType)
	if !strings.Contains(d.Msg, "==") {
		t.Fatalf("GX4007 message = %q", d.Msg)
	}
}
