package compiler_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

// TestREQ_DEV_04_SymbolTableFollowsRouteTypes checks that one generate gives
// a current dev symbol table after a route type loses its generated form
// type. The route file on the disk is older than the route type then.
func TestREQ_DEV_04_SymbolTableFollowsRouteTypes(t *testing.T) {
	files := map[string]string{
		"go.mod":         moduleWithGx(t),
		"cart/routes.go": signalRoute,
		"cart/action.go": addAction,
		"cart/Cart.gx":   "package cart\n\nsignals {\n  Qty int = 1\n}\n\n<button on:click={Add{}}>Add</button>\n",
	}
	dir := writeTree(t, files)
	write := func() {
		t.Helper()
		for path, src := range generateFiles(t, dir) {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, src, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	write()
	if diags := compiler.Stale(dir); len(diags) != 0 {
		t.Fatalf("stale after the first generate: %v", diags)
	}
	// The action reads no signal now, so its route has no form type.
	if err := os.WriteFile(filepath.Join(dir, "cart/routes.go"), []byte(addRoute), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cart/Cart.gx"), []byte("package cart\n\n<button on:click={Add{ID: 1}}>Add</button>\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	write()
	if diags := compiler.Stale(dir); len(diags) != 0 {
		t.Fatalf("stale after one generate of the changed route: %v", diags)
	}
}
