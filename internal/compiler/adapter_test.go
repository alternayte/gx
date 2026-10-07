package compiler_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
	"github.com/alternayte/gx/internal/gxconfig"
)

const htmxToml = "adapter = \"htmx\"\n"

// TestREQ_ACT_09_SignalsUnderHtmx checks that a signal, a client expression
// and a signal-bound action field are compile errors under the htmx
// adapter, and that the same files compile under Datastar.
func TestREQ_ACT_09_SignalsUnderHtmx(t *testing.T) {
	for _, tc := range []struct {
		name, gx, route, want string
		line                  int
	}{
		{"signals block", "package cart\n\nsignals {\n  Qty int = 1\n}\n\n<p>cart</p>\n", addRoute, "declares signal \"Qty\"", 4},
		{"show", "package cart\n\nsignals {\n  Open bool = false\n}\n\n<p show={$Open}>cart</p>\n", addRoute, "the \"show\" directive is a client expression", 7},
		{"bind", "package cart\n\nsignals {\n  Name string = \"\"\n}\n\n<input bind:value={$Name}/>\n", addRoute, "the \"bind\" directive is a client expression", 7},
		{"class", "package cart\n\nsignals {\n  Open bool = false\n}\n\n<p class:ring-2={$Open}>cart</p>\n", addRoute, "the \"class\" directive is a client expression", 7},
		{"signal statement", "package cart\n\nsignals {\n  Qty int = 1\n}\n\n<button on:click={$Qty++}>Add</button>\n", addRoute, "this on: handler holds signal statements", 7},
		{"modifier", "package cart\n\n<button on:click.outside={Add{ID: 1}}>Add</button>\n", addRoute, "it has no \".outside\" modifier", 3},
		{"signal field", "package cart\n\n<button on:click={Add{}}>Add</button>\n", signalRoute, "field \"Qty\" of the action reads signal \"qty\"", 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := map[string]string{
				"go.mod":         moduleWithGx(t),
				"gx.toml":        htmxToml,
				"cart/routes.go": tc.route,
				"cart/action.go": addAction,
				"cart/Cart.gx":   tc.gx,
			}
			dir := writeTree(t, files)
			var found bool
			for _, d := range checkDir(t, dir) {
				if d.Code != compiler.CodeAdapterSignals {
					continue
				}
				if strings.Contains(d.Msg, tc.want) {
					found = true
					if !strings.HasPrefix(d.Msg, "the htmx adapter ") || d.Line != tc.line || !strings.HasSuffix(d.File, "Cart.gx") {
						t.Fatalf("GX4006 = %s:%d %q, want Cart.gx:%d", d.File, d.Line, d.Msg, tc.line)
					}
				}
			}
			if !found {
				t.Fatalf("no GX4006 with %q in %v", tc.want, checkDir(t, dir))
			}
			if tc.name == "signal field" {
				return // the field has no signal under Datastar either (GX4003)
			}
			files["gx.toml"] = "adapter = \"datastar\"\n"
			for _, d := range checkDir(t, writeTree(t, files)) {
				if d.Code == compiler.CodeAdapterSignals {
					t.Fatalf("GX4006 under datastar: %v", d)
				}
			}
		})
	}
}

// TestREQ_ACT_09_ActionCompilesUnderHtmx checks that an action invocation
// with the modifiers that htmx has compiles under the htmx adapter, and
// that the generated code is the code of a Datastar app.
func TestREQ_ACT_09_ActionCompilesUnderHtmx(t *testing.T) {
	files := map[string]string{
		"go.mod":         moduleWithGx(t),
		"gx.toml":        htmxToml,
		"cart/routes.go": addRoute,
		"cart/action.go": addAction,
		"cart/Cart.gx":   "package cart\n\n<button on:click.once.debounce(300ms)={Add{ID: 1}}>Add</button>\n<div on:load={Add{ID: 2}}></div>\n",
	}
	dir := writeTree(t, files)
	if diags := checkDir(t, dir); len(diags) != 0 {
		t.Fatalf("unexpected diagnostics %v", diags)
	}
	htmx := string(generateFiles(t, dir)[filepath.Join(dir, "cart/Cart_gx.go")])
	if want := `gx.On("click.once.debounce(300ms)", "POST", (Add{ID: 1}).URL(), "")`; !strings.Contains(htmx, want) {
		t.Fatalf("Cart_gx.go lacks %s:\n%s", want, htmx)
	}
	delete(files, "gx.toml")
	other := writeTree(t, files)
	datastar := string(generateFiles(t, other)[filepath.Join(other, "cart/Cart_gx.go")])
	if htmx != datastar {
		t.Fatalf("the generated code differs between the adapters:\nhtmx:\n%s\ndatastar:\n%s", htmx, datastar)
	}
}

// TestREQ_ACT_09_UnknownAdapter checks that gx.toml refuses an adapter name
// that Gx does not have.
func TestREQ_ACT_09_UnknownAdapter(t *testing.T) {
	dir := writeTree(t, map[string]string{"gx.toml": "adapter = \"alpine\"\n"})
	if _, err := gxconfig.Load(dir); err == nil || !strings.Contains(err.Error(), `unknown adapter "alpine"`) {
		t.Fatalf("err = %v, want an unknown adapter error", err)
	}
}
