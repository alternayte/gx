package compiler_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

const optimisticButton = `package cart

signals {
  Count int = 0
  Busy bool = false
}

<div>
  <button on:click={Add{ID: 1}} optimistic:click={$Count++; $Busy = true}>Add</button>
  <span text={$Count}></span>
</div>
`

// TestREQ_ACT_18_OptimisticCodegen checks the generated code of an
// optimistic directive: a capture handler of the same event that gives the
// runtime the value of each signal that the statements write, then runs the
// statements. The action of the element is marked, so the adapter does not
// send a failed request again (REQ-ACT-18).
func TestREQ_ACT_18_OptimisticCodegen(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":         moduleWithGx(t),
		"cart/routes.go": addRoute,
		"cart/action.go": addAction,
		"cart/Button.gx": optimisticButton,
	})
	src := string(generateFiles(t, dir)[filepath.Join(dir, "cart/Button_gx.go")])
	for _, want := range []string{
		`gx.Client("data-on:click__capture", gx.Keep(gx.KeepSignal("cart.Button", p.GxKey, "count"), gx.KeepSignal("cart.Button", p.GxKey, "busy"))+gx.SignalPath("cart.Button", p.GxKey, "count")+"++"`,
		`gx.ExprOp("keep", `,
		`gx.On("click.optimistic", "POST", (Add{ID: 1}).URL(), `,
	} {
		if !strings.Contains(src, want) {
			t.Errorf("Button_gx.go lacks %s", want)
		}
	}
	if t.Failed() {
		t.Logf("Button_gx.go:\n%s", src)
	}
}

// TestREQ_ACT_18_OptimisticNeedsAnAction checks GX4013: an optimistic
// directive with no action of the same event on the element, and one whose
// statements write no signal (REQ-ACT-18).
func TestREQ_ACT_18_OptimisticNeedsAnAction(t *testing.T) {
	for _, c := range []struct {
		name, button, want string
	}{
		{"no handler", strings.Replace(optimisticButton, `on:click={Add{ID: 1}} `, ``, 1), "no on:click handler that invokes an action"},
		{"a handler of a different event", strings.Replace(optimisticButton, `on:click={Add{ID: 1}}`, `on:focus={Add{ID: 1}}`, 1), "no on:click handler that invokes an action"},
		{"a handler with signal statements only", strings.Replace(optimisticButton, `on:click={Add{ID: 1}}`, `on:click.once={$Busy = false}`, 1), "no on:click handler that invokes an action"},
	} {
		dir := writeTree(t, map[string]string{
			"go.mod":         moduleWithGx(t),
			"cart/routes.go": addRoute,
			"cart/action.go": addAction,
			"cart/Button.gx": c.button,
		})
		_, diags := compiler.Generate(dir)
		found := false
		for _, d := range diags {
			if d.Code == compiler.CodeOptimistic && strings.Contains(d.Msg, c.want) && d.Line == 9 {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: diagnostics = %v, want GX4013 at line 9 with %q", c.name, diags, c.want)
		}
	}
}
