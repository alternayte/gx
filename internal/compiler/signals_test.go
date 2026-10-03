package compiler_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

func TestREQ_AUT_15_ServerSignalRejected(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":          moduleWithGx(t),
		"ui/cart/Cart.gx": "package cart\n\nsignals {\n  Qty int = 1\n}\n\n<p>{$Qty}</p>\n<p title={$Qty}>x</p>\n<input bind:value={$Qty} />\n",
	})
	diags := checkDir(t, dir)
	count := 0
	for _, d := range diags {
		if d.Code == compiler.CodeSignal {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("GX2010 count = %d, want 2: %v", count, diags)
	}
}

func TestREQ_AUT_15_SignalsStruct(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":          moduleWithGx(t),
		"ui/cart/Cart.gx": "package cart\n\nsignals {\n  Qty  int  = 1\n  Open bool = false\n}\n\n<p>x</p>\n",
	})
	files := generateFiles(t, dir)
	src := string(files[filepath.Join(dir, "ui/cart/Cart_gx.go")])
	for _, want := range []string{"type CartSignals struct {", "Qty  int", "Open bool"} {
		if !strings.Contains(src, want) {
			t.Fatalf("Cart_gx.go lacks %q:\n%s", want, src)
		}
	}
}
