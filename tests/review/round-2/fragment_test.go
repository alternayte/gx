package round2_test

import "testing"

// nestedApp is a keyed signal component (Cart) that holds one more signal
// component (Line) with a fragment. Line has one call site and no loop, so
// it needs no key of its own (REQ-ACT-06).
var nestedApp = map[string]string{
	"cart/route/route.go": "package route\n\nimport \"github.com/alternayte/gx\"\n\ntype Bump struct {\n\tgx.Route `POST /bump`\n}\n",
	"cart/Line.gx": `package cart

import "app/cart/route"

props {
  Total int
}

signals {
  Qty int = 1
}

<div>
  total := p.Total
  <span #total(total int)>{total}</span>
  <input type="number" bind:value={$Qty} />
  <button on:click={route.Bump{}}>Bump</button>
</div>
`,
	"cart/Cart.gx": `package cart

props {
  Num int
}

signals {
  Open bool = false
}

<section>
  <button on:click={$Open = !$Open}>Toggle</button>
  <Line total={3} />
</section>
`,
	"cart/Page.gx": `package cart

props {
  Num int
}

<main>
  <Cart key={p.Num} num={p.Num} />
</main>
`,
	"cart/actions.go": `package cart

import (
	"app/cart/route"

	"github.com/alternayte/gx"
)

// bump patches the total of the Line instance that invoked it.
var bump = gx.Action(func(c *gx.Ctx, in route.Bump) error {
	return c.Patch(LineTotal(gx.ScopeKey(gx.Scope(c.R), "cart.Line"), 9))
})

var Routes = gx.Collect(bump)
`,
	"cart/patch_test.go": `package cart

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/alternayte/gx"
	"github.com/alternayte/gx/adapters/datastar"
)

func between(s, open, end string) string {
	_, rest, ok := strings.Cut(s, open)
	if !ok {
		return ""
	}
	out, _, _ := strings.Cut(rest, end)
	return out
}

// cssIdent reads a CSS identifier with its escapes. ok is false when a
// character of s cannot be part of an identifier.
func cssIdent(s string) (string, bool) {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '\\':
			i++
			if i >= len(s) {
				return "", false
			}
			j := i
			for j < len(s) && j-i < 6 && strings.IndexByte("0123456789abcdefABCDEF", s[j]) >= 0 {
				j++
			}
			if j == i {
				b.WriteByte(s[i])
				continue
			}
			n, _ := strconv.ParseInt(s[i:j], 16, 32)
			b.WriteRune(rune(n))
			if j < len(s) && s[j] == ' ' {
				j++
			}
			i = j - 1
		case c == '-' || c == '_' || c >= 0x80 || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z':
			b.WriteByte(c)
		default:
			return "", false
		}
	}
	return b.String(), true
}

// selects reports whether the patch selector names the element with the
// id: no selector (Datastar then takes the id of the element it gets), an
// id selector or an attribute selector.
func selects(selector, id string) bool {
	if selector == "" {
		return true
	}
	if rest, ok := strings.CutPrefix(selector, "#"); ok {
		got, ok := cssIdent(rest)
		return ok && got == id
	}
	for _, q := range []string{"\"", "'"} {
		if rest, ok := strings.CutPrefix(selector, "[id="+q); ok {
			if value, ok := strings.CutSuffix(rest, q+"]"); ok {
				return strings.NewReplacer("\\"+q, q, "\\\\", "\\").Replace(value) == id
			}
		}
	}
	return false
}

func TestInnerNestedFragmentPatch(t *testing.T) {
	page := gx.String(Page(PageProps{Num: 42}))
	id := between(page, "<span id=\"", "\"")
	scope := "cart.Line" + between(page, "data-gx-instance=\"cart.Line", "\"")
	if id == "" || scope == "cart.Line" {
		t.Fatalf("no fragment id or no Line scope in the page:\n%s", page)
	}
	app := gx.New(gx.Config{Adapter: datastar.Adapter()})
	app.Group("/", Routes)
	srv := httptest.NewServer(app)
	defer srv.Close()
	req, err := http.NewRequest("POST", srv.URL+"/bump", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Gx-Scope", scope)
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Accept", "text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	if resp.StatusCode != 200 || !strings.Contains(body, "id=\""+id+"\"") {
		t.Fatalf("status %d; the answer does not patch the element %q:\n%s", resp.StatusCode, id, body)
	}
	selector := ""
	for _, line := range strings.Split(body, "\n") {
		if rest, ok := strings.CutPrefix(line, "data: selector "); ok {
			selector = rest
		}
	}
	if !selects(selector, id) {
		t.Fatalf("the patch selector %q does not select the element with id %q; document.querySelectorAll(%q) throws or finds another element", selector, id, selector)
	}
}
`,
}

// TestREQ_ACT_14_FragmentPatchReachesNestedInstance: a signal component
// with one call site inside a keyed signal component gets the generated
// instance key "<parent key>.<call site>". Its fragment id holds that key,
// and c.Patch sends the CSS selector "#" + id with no escape. The dot
// starts a class selector that begins with a digit, so the selector is not
// valid CSS and Datastar cannot apply the patch (REQ-ACT-14, REQ-ACT-04:
// "sends fragment patches by id"). The same defect breaks a key that the
// app supplies with a dot, a space or another selector character.
//
// The test accepts an answer with no selector, an id selector with CSS
// escapes or an attribute selector. It does not need a browser.
func TestREQ_ACT_14_FragmentPatchReachesNestedInstance(t *testing.T) {
	dir := scratchModule(t, nestedApp)
	generateInto(t, dir)
	out := goTest(dir, "TestInnerNestedFragmentPatch")
	if !innerPassed(out, "TestInnerNestedFragmentPatch") {
		t.Fatalf("the generated app does not patch the fragment of the nested instance:\n%s", tail(out, 12))
	}
}
