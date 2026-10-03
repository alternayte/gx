package compiler_test

import (
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

func TestREQ_AUT_14_LoopKey(t *testing.T) {
	base := "package cart\n\nprops {\n  Items []string\n}\n\n<ul>\n  for _, it := range p.Items {\n    <li><input value={it}></li>\n  }\n</ul>\n"
	dir := writeTree(t, map[string]string{
		"go.mod":          moduleWithGx(t),
		"ui/cart/Cart.gx": base,
	})
	if !hasCode(checkDir(t, dir), compiler.CodeLoopKey) {
		t.Fatalf("input in an unkeyed loop: want GX2009")
	}

	keyed := strings.Replace(base, "<li><input value={it}></li>", "<li key={it}><input value={it}></li>", 1)
	dir = writeTree(t, map[string]string{
		"go.mod":          moduleWithGx(t),
		"ui/cart/Cart.gx": keyed,
	})
	if diags := checkDir(t, dir); len(diags) != 0 {
		t.Fatalf("keyed loop: unexpected diagnostics %v", diags)
	}

	frag := strings.Replace(base, "<li><input value={it}></li>", "<li #row(key string, it string)><input value={it}></li>", 1)
	dir = writeTree(t, map[string]string{
		"go.mod":          moduleWithGx(t),
		"ui/cart/Cart.gx": frag,
	})
	if diags := checkDir(t, dir); len(diags) != 0 {
		t.Fatalf("keyed fragment: unexpected diagnostics %v", diags)
	}
}

func TestREQ_AUT_14_SignalComponentNeedsKey(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":          moduleWithGx(t),
		"ui/cart/Cart.gx": "package cart\n\nsignals {\n  Qty int = 1\n}\n\n<p>x</p>\n",
		"ui/cart/List.gx": "package cart\n\nprops {\n  Items []string\n}\n\nfor _, it := range p.Items {\n  <Cart />\n}\n",
	})
	if !hasCode(checkDir(t, dir), compiler.CodeLoopKey) {
		t.Fatalf("signal component in an unkeyed loop: want GX2009")
	}
}
