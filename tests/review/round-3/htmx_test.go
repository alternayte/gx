package round3_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

// REQ-ACT-09: one build runs under Datastar and under htmx, and what htmx
// cannot express is a compile error (GX4006).
//
// Defect: an element with two on: handlers that invoke actions compiles
// with no diagnostic under htmx and renders hx-post and hx-trigger two
// times each. An HTML parser keeps the first attribute of a name, so the
// browser drops the second action with no error. Under Datastar the two
// handlers are data-on:click and data-on:keydown, and both run.
func TestREQ_ACT_09_TwoActionsOnOneElementUnderHtmx(t *testing.T) {
	dir := scratchModule(t, map[string]string{
		"gx.toml": "adapter = \"htmx\"\n",
		"cart/routes.go": "package cart\n\nimport \"github.com/alternayte/gx\"\n\n" +
			"type Add struct {\n\tgx.Route `POST /cart/add`\n\tID int64\n}\n\n" +
			"type Remove struct {\n\tgx.Route `POST /cart/remove`\n\tID int64\n}\n",
		"cart/action.go": "package cart\n\nimport \"github.com/alternayte/gx\"\n\n" +
			"var add = gx.Action(func(c *gx.Ctx, in Add) error { return nil })\n" +
			"var remove = gx.Action(func(c *gx.Ctx, in Remove) error { return nil })\n",
		"cart/Cart.gx": "package cart\n\n<button on:click={Add{ID: 1}} on:contextmenu={Remove{ID: 1}}>Item</button>\n",
		"main.go": "package main\n\nimport (\n\t\"fmt\"\n\n\t\"app/cart\"\n\tgx \"github.com/alternayte/gx\"\n\t\"github.com/alternayte/gx/adapters/htmx\"\n)\n\n" +
			"func main() {\n\tgx.SetAdapter(htmx.Adapter())\n\tfmt.Println(gx.String(cart.Cart(cart.CartProps{})))\n}\n",
	})
	for _, d := range compiler.Check(dir) {
		if d.Code == compiler.CodeAdapterSignals {
			return // the compiler refuses what htmx cannot express
		}
	}
	generateInto(t, dir)
	out := strings.TrimSpace(goRun(t, dir, "."))
	if !strings.HasPrefix(out, "<button") {
		t.Fatalf("the app did not render the button:\n%s", out)
	}
	tag, _, _ := strings.Cut(out, ">")
	seen := map[string]bool{}
	for _, m := range regexp.MustCompile(` ([a-zA-Z0-9:_-]+)=`).FindAllStringSubmatch(tag, -1) {
		if seen[m[1]] {
			t.Fatalf("no GX4006, and the element has the attribute %s two times, so the browser drops the second action: %s", m[1], out)
		}
		seen[m[1]] = true
	}
	if !strings.Contains(tag, "/cart/add") || !strings.Contains(tag, "/cart/remove") {
		t.Fatalf("no GX4006, and the element does not invoke both actions: %s", out)
	}
}
