package compiler_test

import (
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

// TestREQ_ACT_17_UpdateWithNoFragmentIsAHint checks the hint for a component
// that c.Update takes, that has no fragment and that has many dynamic
// values: the server then sends the whole root for each change. A component
// with a fragment, and a small component, have no hint (REQ-ACT-17).
func TestREQ_ACT_17_UpdateWithNoFragmentIsAHint(t *testing.T) {
	values := strings.Repeat("<td>{p.A}</td>", 9)
	dir := writeTree(t, map[string]string{
		"go.mod":           moduleWithGx(t),
		"shop/route/r.go":  "package route\n\nimport \"github.com/alternayte/gx\"\n\ntype Save struct {\n\tgx.Route `POST /save`\n}\n",
		"shop/Big.gx":      "package shop\n\nprops {\n  A string\n}\n\n<table id=\"big\"><tr>" + values + "</tr></table>\n",
		"shop/Small.gx":    "package shop\n\nprops {\n  A string\n}\n\n<p id=\"small\">{p.A}</p>\n",
		"shop/WithFrag.gx": "package shop\n\nprops {\n  A string\n}\n\n<table id=\"frag\"><tr #row>" + values + "</tr></table>\n",
		"shop/shop.go": `package shop

import (
	"app/shop/route"

	"github.com/alternayte/gx"
)

var Save = gx.Action(func(c *gx.Ctx, in route.Save) error {
	if err := c.Update(Small(SmallProps{A: "x"})); err != nil {
		return err
	}
	if err := c.Update(WithFrag(WithFragProps{A: "x"})); err != nil {
		return err
	}
	return c.Update(Big(BigProps{A: "x"}))
})

var Routes = gx.Collect(Save)
`,
		"main.go": "package main\n\nimport (\n\t\"app/shop\"\n\n\t\"github.com/alternayte/gx\"\n)\n\nfunc main() {\n\tgx.New(gx.Config{}).Group(\"/\", shop.Routes)\n}\n",
	})
	writeGenerated(t, dir)
	diags := compiler.CheckApp(dir, compiler.CheckOptions{})
	if len(diags) != 1 {
		t.Fatalf("diagnostics = %v, want one hint", diags)
	}
	d := diags[0]
	if d.Code != compiler.CodeUpdateNoFragment || !d.Hint {
		t.Errorf("diagnostic = %+v, want the hint GX4012", d)
	}
	if !strings.HasSuffix(d.File, "shop.go") || d.Line != 16 {
		t.Errorf("the hint is at %s:%d, want the c.Update call of Big at shop.go:16", d.File, d.Line)
	}
	if !strings.Contains(d.Msg, "Big") || !strings.Contains(d.Msg, "fragment") {
		t.Errorf("the message does not name the component and the cause: %s", d.Msg)
	}
	if compiler.Failed(diags) {
		t.Error("a hint fails the check")
	}
	if !compiler.Failed([]compiler.Diagnostic{{Code: compiler.CodeStale}}) {
		t.Error("an error does not fail the check")
	}
}
