package gx_test

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/alternayte/gx"
)

// What the compiler writes for a cart with three kinds of fragment:
//
//	<div #cart><span #total>{total}</span><ul #rows>{rows}</ul></div>
//	<li #row(key)>{name}</li>
var (
	cartFragments = gx.NewTemplate(
		[]string{"<div", "><span", ">", "</span><ul", ">", "</ul></div>"},
		[]int{0, 1, 2, 1, 2},
		[]gx.TemplateEl{
			{Slot: 0, Start: 0, EndStatic: 5, End: len("</ul></div>")},
			{Slot: 1, Start: 1, EndStatic: 3, End: len("</span>")},
			{Slot: 3, Start: len("</span>"), EndStatic: 5, End: len("</ul>")},
		},
		[]gx.TemplateRoot{{El: 0, Slot: -1}},
	)
	rowFragment = gx.NewTemplate(
		[]string{"<li", ">", "</li>"},
		[]int{0, 1},
		[]gx.TemplateEl{{Slot: 0, Start: 0, EndStatic: 2, End: len("</li>")}},
		[]gx.TemplateRoot{{El: 0, Slot: -1}},
	)
)

func fragID(id string) gx.Attrs { return gx.Attrs{{Key: "id", Value: id}} }

// cartNode is the cart with one row for each name.
func cartNode(total string, names ...string) gx.Node {
	var rows gx.Builder
	for i, name := range names {
		rows.Add(rowFragment.With(gx.OpenFragment("li", fragID("row-"+string(rune('1'+i)))), gx.Text(name)))
	}
	return cartFragments.With(
		gx.OpenFragment("div", fragID("cart")),
		gx.OpenFragment("span", fragID("total")),
		gx.Text(total),
		gx.OpenFragment("ul", fragID("rows")),
		rows.Node(),
	)
}

var fragHashRE = regexp.MustCompile(`id="([^"]+)" data-gx-h="([0-9a-f]{8})"`)

// fragHashes returns the hash of each fragment of a page, by id.
func fragHashes(t *testing.T, n gx.Node) map[string]string {
	t.Helper()
	html := gx.StringRequest(httptest.NewRequest("GET", "/cart", nil), n)
	out := map[string]string{}
	for _, m := range fragHashRE.FindAllStringSubmatch(html, -1) {
		out[m[1]] = m[2]
	}
	return out
}

// changed returns the ids whose hash differs between two renders.
func changed(a, b map[string]string) string {
	var ids []string
	for _, id := range []string{"cart", "total", "rows", "row-1", "row-2", "row-3"} {
		if a[id] != b[id] {
			ids = append(ids, id)
		}
	}
	return strings.Join(ids, " ")
}

// TestREQ_ACT_15_FragmentHash checks the hash of a fragment: a render with a
// request writes it, equal props give equal hashes, and a changed value
// changes the hash of its own fragment only (REQ-ACT-15).
func TestREQ_ACT_15_FragmentHash(t *testing.T) {
	first := cartNode("10", "tea", "milk")
	if html := gx.String(first); strings.Contains(html, "data-gx-h") {
		t.Errorf("a render with no request has a hash: %s", html)
	}
	html := gx.StringRequest(httptest.NewRequest("GET", "/cart", nil), first)
	want := regexp.MustCompile(`^<div id="cart" data-gx-h="[0-9a-f]{8}"><span id="total" data-gx-h="[0-9a-f]{8}">10</span><ul id="rows" data-gx-h="[0-9a-f]{8}"><li id="row-1" data-gx-h="[0-9a-f]{8}">tea</li><li id="row-2" data-gx-h="[0-9a-f]{8}">milk</li></ul></div>$`)
	if !want.MatchString(html) {
		t.Fatalf("page with fragments:\n%s", html)
	}

	base := fragHashes(t, first)
	if len(base) != 5 {
		t.Fatalf("hashes = %v, want one for each of the 5 fragments", base)
	}
	if got := changed(base, fragHashes(t, cartNode("10", "tea", "milk"))); got != "" {
		t.Errorf("equal props changed the hash of: %s", got)
	}
	for _, c := range []struct {
		name string
		n    gx.Node
		want string
	}{
		{"a new total", cartNode("12", "tea", "milk"), "total"},
		{"a new text of one row", cartNode("10", "tea", "oat milk"), "row-2"},
		{"one more row", cartNode("10", "tea", "milk", "rice"), "rows row-3"},
		{"one row less", cartNode("10", "tea"), "rows row-2"},
	} {
		if got := changed(base, fragHashes(t, c.n)); got != c.want {
			t.Errorf("%s changed the hash of %q, want %q", c.name, got, c.want)
		}
	}
}

// TestREQ_ACT_16_Update checks that c.Update sends only the fragments whose
// hash differs from the hash of the request (REQ-ACT-16).
func TestREQ_ACT_16_Update(t *testing.T) {
	have := fragHashes(t, cartNode("10", "tea", "milk"))
	header := func(skip string) string {
		var parts []string
		for id, hash := range have {
			if id != skip {
				parts = append(parts, id+"="+hash)
			}
		}
		return strings.Join(parts, ",")
	}
	update := func(n gx.Node, fragments string) (*fakeAdapter, *httptest.ResponseRecorder) {
		a := &fakeAdapter{}
		app := gx.New(gx.Config{Adapter: a})
		app.Group("/", gx.Collect(gx.Action(func(c *gx.Ctx, in actRoute) error { return c.Update(n) })))
		req := httptest.NewRequest("POST", "/act", nil)
		if fragments != "" {
			req.Header.Set("Gx-Fragments", fragments)
		}
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, req)
		return a, rec
	}
	targets := func(a *fakeAdapter) string {
		var out []string
		if a.res == nil {
			return ""
		}
		for _, p := range a.res.Patches {
			ep, ok := p.(gx.ElementPatch)
			if !ok {
				t.Fatalf("patch = %#v, want an element patch", p)
			}
			out = append(out, ep.Target)
		}
		return strings.Join(out, " ")
	}

	for _, c := range []struct {
		name      string
		n         gx.Node
		fragments string
		want      string
	}{
		{"one row changed", cartNode("10", "tea", "oat milk"), header(""), "#row-2"},
		{"the total and one row changed", cartNode("12", "tea", "oat milk"), header(""), "#total #row-2"},
		{"a new row: the list goes, with its rows", cartNode("10", "tea", "milk", "rice"), header(""), "#rows"},
		{"the browser has no hash of one fragment", cartNode("10", "tea", "milk"), header("total"), "#total"},
		{"a request with no hashes gets the outer fragment", cartNode("10", "tea", "milk"), "", "#cart"},
	} {
		a, _ := update(c.n, c.fragments)
		if got := targets(a); got != c.want {
			t.Errorf("%s: patches of %q, want %q", c.name, got, c.want)
		}
	}

	// The patch holds the HTML of the fragment, with its new hash.
	a, _ := update(cartNode("10", "tea", "oat milk"), header(""))
	row := gx.StringRequest(httptest.NewRequest("POST", "/act", nil), a.res.Patches[0].(gx.ElementPatch).Node)
	if !regexp.MustCompile(`^<li id="row-2" data-gx-h="[0-9a-f]{8}">oat milk</li>$`).MatchString(row) {
		t.Errorf("patch of the row: %s", row)
	}
	if strings.Contains(row, have["row-2"]) {
		t.Errorf("the patch has the hash of before: %s", row)
	}

	// No change: no patch, so the action answers 204 (REQ-ACT-10).
	if a, rec := update(cartNode("10", "tea", "milk"), header("")); targets(a) != "" || rec.Code != http.StatusNoContent {
		t.Errorf("no change: patches %q and status %d, want none and 204", targets(a), rec.Code)
	}

	// A node with no fragment goes as its root, as c.Patch sends it.
	a, _ = update(gx.El("p", fragID("note"), gx.Text("x")), header(""))
	if got := targets(a); got != "#note" {
		t.Errorf("a node with no fragment: patches of %q, want #note", got)
	}
}
