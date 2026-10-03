package compiler_test

import (
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

const signalledRoute = `package cart

import "github.com/alternayte/gx"

type Add struct {
	gx.Route ` + "`" + `POST /cart/add` + "`" + `
	Qty int ` + "`" + `signal:"qty"` + "`" + `
}
`

const signalledAction = `package cart

import "github.com/alternayte/gx"

var add = gx.Action(func(c *gx.Ctx, in Add) error { return nil })
`

// TestSI_13_MissingRules checks GX4008 for a signal-bound field with no
// Rules.
func TestSI_13_MissingRules(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":         moduleWithGx(t),
		"cart/routes.go": signalledRoute,
		"cart/action.go": signalledAction,
		"cart/Cart.gx":   "package cart\n\nsignals {\n  Qty int = 1\n}\n\n<button on:click={Add{}}>x</button>\n",
	})
	diags := checkDir(t, dir)
	d := diagWith(t, diags, compiler.CodeSignalRules)
	if !strings.Contains(d.Msg, "Rules()") {
		t.Fatalf("GX4008 message = %q", d.Msg)
	}
}

// TestSI_13_RulesMethod passes the analyzer.
func TestSI_13_RulesMethod(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":         moduleWithGx(t),
		"cart/routes.go": signalledRoute,
		"cart/action.go": signalledAction,
		"cart/rules.go": "package cart\n\nimport \"github.com/alternayte/gx\"\n\nfunc (in *Add) Rules() gx.Rules {\n\treturn gx.Rules{gx.Field(&in.Qty, gx.Min(1), gx.Max(5))}\n}\n",
		"cart/Cart.gx":   "package cart\n\nsignals {\n  Qty int = 1\n}\n\n<button on:click={Add{}}>x</button>\n",
	})
	for _, d := range checkDir(t, dir) {
		if d.Code == compiler.CodeSignalRules {
			t.Fatalf("Rules() still reports GX4008: %v", d)
		}
	}
}

// TestSI_13_UncheckedMarker passes the analyzer.
func TestSI_13_UncheckedMarker(t *testing.T) {
	route := strings.Replace(signalledRoute, "type Add struct {", "type Add struct {\n\tgx.Unchecked", 1)
	dir := writeTree(t, map[string]string{
		"go.mod":         moduleWithGx(t),
		"cart/routes.go": route,
		"cart/action.go": signalledAction,
		"cart/Cart.gx":   "package cart\n\nsignals {\n  Qty int = 1\n}\n\n<button on:click={Add{}}>x</button>\n",
	})
	for _, d := range checkDir(t, dir) {
		if d.Code == compiler.CodeSignalRules {
			t.Fatalf("gx.Unchecked still reports GX4008: %v", d)
		}
	}
}
