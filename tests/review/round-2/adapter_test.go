package round2_test

import (
	"html"
	"strings"
	"testing"

	"github.com/alternayte/gx"
	"github.com/alternayte/gx/adapters/datastar"
)

// codeOutsideStrings returns the parts of a JavaScript expression that are
// not inside a string literal. ok is false when a literal has no end.
func codeOutsideStrings(expr string) (code string, ok bool) {
	var b strings.Builder
	for i := 0; i < len(expr); i++ {
		c := expr[i]
		if c != '\'' && c != '"' && c != '`' {
			b.WriteByte(c)
			continue
		}
		j := i + 1
		for j < len(expr) && expr[j] != c {
			if expr[j] == '\\' {
				j++
			}
			j++
		}
		if j >= len(expr) {
			return b.String(), false
		}
		i = j
	}
	return b.String(), true
}

// TestSI_05_InstanceKeyInActionInvocationIsEscaped: an action invocation
// of a keyed component holds the signal scope, and the scope holds the
// instance key, which is a server value (key={p.Name}). Datastar runs the
// attribute as JavaScript. The Datastar adapter writes the scope between
// single quotes with no escape, so a key with a quote ends the string and
// the rest of the key runs as code (SI-05).
func TestSI_05_InstanceKeyInActionInvocationIsEscaped(t *testing.T) {
	gx.SetAdapter(datastar.Adapter())
	for _, key := range []string{
		`x'}}); alert(1); ({a:{b:'`,
		`x\'}}); alert(1); ({a:{b:\'`,
	} {
		scope := gx.ScopeString("cart.Cart", gx.InstanceKey(key))
		var out string
		rejected := func() (rejected bool) {
			// A fix can refuse the key. A panic is a refusal.
			defer func() { rejected = recover() != nil }()
			out = gx.String(gx.El("button", gx.Attrs{gx.Invoke("POST", "/cart/add", scope)}, gx.Text("Add")))
			return false
		}()
		if rejected {
			continue
		}
		_, rest, found := strings.Cut(out, `="`)
		if !found {
			t.Fatalf("no attribute in %s", out)
		}
		value, _, _ := strings.Cut(rest, `"`)
		// The browser decodes the attribute before Datastar reads it.
		expr := html.UnescapeString(value)
		code, ok := codeOutsideStrings(expr)
		if !ok || strings.Contains(code, "alert") {
			t.Errorf("key %q: the key leaves the string literal and runs as code:\n  expression: %s\n  code part:  %s", key, expr, code)
		}
	}
}
