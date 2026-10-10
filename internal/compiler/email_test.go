package compiler_test

import (
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

// TestREQ_REG_17_EmailPackage checks GX6010: each construct that an email
// cannot hold is a compile error in a component of an email package, and
// the same markup in a different package compiles (REQ-REG-17).
func TestREQ_REG_17_EmailPackage(t *testing.T) {
	const head = "props {\n  Title string\n  Href gx.URL\n}\n\n"
	generate := func(pkg, extra, body string) []compiler.Diagnostic {
		t.Helper()
		files := map[string]string{
			"go.mod":                     moduleWithGx(t),
			"ui/" + pkg + "/Part.gx":     "package " + pkg + "\n\n" + extra + head + body + "\n",
			"cart/route/route.go":        "package route\n\nimport \"github.com/alternayte/gx\"\n\ntype Add struct {\n\tgx.Route `POST /cart/add`\n}\n",
			"cart/cart.go":               "package cart\n\nimport (\n\t\"github.com/alternayte/gx\"\n\t\"app/cart/route\"\n)\n\nvar Add = gx.Action(func(c *gx.Ctx, in route.Add) error { return nil })\n\nvar Routes = gx.Collect(Add)\n",
			"ui/" + pkg + "/Chart.ts":    "import { island } from 'gx'\n\nexport default island<{ title: string }>((el, props) => {\n  el.textContent = props.title\n})\n",
			"ui/" + pkg + "/chart.go":    "package " + pkg + "\n\n// ChartProps are the props of the island Chart.\ntype ChartProps struct {\n\tTitle string `json:\"title\"`\n}\n",
			"ui/" + pkg + "/placeholder": "",
		}
		dir := writeTree(t, files)
		_, diags := compiler.NewSession().Generate(dir)
		var out []compiler.Diagnostic
		for _, d := range diags {
			if d.Code == "GX6010" {
				out = append(out, d)
			} else if !d.Hint {
				t.Logf("other diagnostic: %s", d.String())
			}
		}
		return out
	}
	for _, c := range []struct {
		name, extra, body, want string
	}{
		{"a class attribute", "", `<p class="text-sm">{p.Title}</p>`, "a class attribute on <p>"},
		{"a dynamic class attribute", "", `<p class={p.Title}>{p.Title}</p>`, "a class attribute on <p>"},
		{"a signal", "signals {\n  Open bool = false\n}\n\n", `<p>{p.Title}</p>`, `the signal "Open"`},
		{"a client expression", "signals {\n  Open bool = false\n}\n\n", `<p show={$Open}>{p.Title}</p>`, "a client expression in show"},
		{"an action invocation", "import \"app/cart/route\"\n\n", `<button on:click={route.Add{}}>Add</button>`, "an action invocation in on:click"},
		{"an island", "", `<div><Chart title={p.Title} /></div>`, "the island <Chart>"},
		{"a script element", "", "<div><script>console.log(1)</script></div>", "a script element"},
	} {
		diags := generate("email", c.extra, c.body)
		found := false
		for _, d := range diags {
			found = found || strings.Contains(d.Msg, c.want)
		}
		if !found {
			t.Errorf("%s in package email: GX6010 findings %v, want one with %q", c.name, diags, c.want)
		}
		if other := generate("card", c.extra, c.body); len(other) != 0 {
			t.Errorf("%s in package card: %v, want no GX6010", c.name, other)
		}
	}
	// Inline styles and a typed link are what an email has.
	if diags := generate("email", "", `<p style="color:var(--primary)"><a href={p.Href}>{p.Title}</a></p>`); len(diags) != 0 {
		t.Errorf("a style and a link: %v", diags)
	}
	// The finding is a compile error: it fails the generate step.
	if diags := generate("email", "", `<p class="a">x</p>`); len(diags) != 1 || !compiler.Failed(diags) || diags[0].Fix == "" || diags[0].Line != 8 {
		t.Errorf("GX6010 is not one error with a fix at the line of the attribute: %+v", diags)
	}
}
