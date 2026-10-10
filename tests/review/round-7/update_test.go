package round7_test

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/alternayte/gx"
)

// recAdapter records the response of an action.
type recAdapter struct{ res *gx.Response }

func (*recAdapter) Name() string              { return "rec" }
func (*recAdapter) Signals() bool             { return true }
func (*recAdapter) Runtime() gx.Node          { return nil }
func (*recAdapter) Assets() map[string][]byte { return nil }
func (a *recAdapter) Respond(w http.ResponseWriter, _ *http.Request, res *gx.Response) error {
	a.res = res
	status := res.Status
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	return nil
}
func (*recAdapter) ReadSignals(*http.Request, any) error { return nil }
func (*recAdapter) Invoke(method, url, scope string) gx.Attr {
	return gx.Attr{Key: "data-rec-on", Value: method + " " + url}
}
func (*recAdapter) On(inv gx.Invocation) []gx.Attr {
	return []gx.Attr{{Key: "data-rec-on-" + inv.Event, Value: inv.Method + " " + inv.URL}}
}

// actIn is a hand-written action input. A generated route type has the same
// methods.
type actIn struct{}

func (actIn) Pattern() string          { return "POST /act" }
func (actIn) Bind(*http.Request) error { return nil }

func idAttr(v string) gx.Attrs { return gx.Attrs{{Key: "id", Value: v}} }

var hashRE = regexp.MustCompile(`id="([^"]+)" data-gx-h="([0-9a-f]{8})"`)

// pageHashes returns the Gx-Fragments header that the runtime sends from a
// page that shows the node: the page is the answer to a GET of path.
func pageHashes(n gx.Node, path string) string {
	html := gx.StringRequest(httptest.NewRequest("GET", path, nil), n)
	var parts []string
	for _, m := range hashRE.FindAllStringSubmatch(html, -1) {
		parts = append(parts, m[1]+"="+m[2])
	}
	return strings.Join(parts, ",")
}

// update runs an action that calls c.Update(n). The request comes from the
// page of path and holds the hashes. It returns the target of each patch and
// the status.
func update(t *testing.T, n gx.Node, path, fragments string) ([]string, int) {
	t.Helper()
	a := &recAdapter{}
	app := gx.New(gx.Config{Adapter: a})
	app.Group("/", gx.Collect(gx.Action(func(c *gx.Ctx, in actIn) error { return c.Update(n) })))
	req := httptest.NewRequest("POST", "/act", nil)
	// The request of a browser from the page of path.
	req.Header.Set("Referer", "http://example.com"+path)
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Gx-Fragments", fragments)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	var out []string
	if a.res != nil {
		for _, p := range a.res.Patches {
			if ep, ok := p.(gx.ElementPatch); ok {
				out = append(out, ep.Target)
			} else {
				out = append(out, "?")
			}
		}
	}
	return out, rec.Code
}

// What the compiler writes for
//
//	<nav #menu><a href={route.Cart{}} active="page">Cart</a></nav>
var menuTmpl = gx.NewTemplate(
	[]string{"<nav", "><a", ">Cart</a></nav>"},
	[]int{0, 1},
	[]gx.TemplateEl{
		{Slot: 0, Start: 0, EndStatic: 2, End: len(">Cart</a></nav>")},
		{Slot: 1, Start: 1, EndStatic: 2, End: len(">Cart</a>")},
	},
	[]gx.TemplateRoot{{El: 0, Slot: -1}},
)

func menu() gx.Node {
	return menuTmpl.With(
		gx.OpenFragment("nav", idAttr("menu")),
		gx.Open("a", gx.Attrs{{Key: "href", Value: "/cart", Kind: gx.AttrURL, Active: "page"}}),
	)
}

// TestREQ_ACT_15_EqualPropsOnThePageAndInTheAction checks the acceptance of
// REQ-ACT-15: "Two renders with equal props give equal hashes", and
// REQ-ACT-16: "The server compares the hash of each fragment with the hash
// of the request and sends each changed fragment as a patch."
//
// The fragment holds a typed link with the active mark. The render of the
// page GET /cart writes aria-current="page" on the link, and the hash of
// the fragment has it. The render of c.Update has the request of the action
// in scope, POST /act: the link is not active there, so the same props give
// a different hash. c.Update then sends the fragment with no change of a
// prop, and the morph removes aria-current from the link of the page.
func TestREQ_ACT_15_EqualPropsOnThePageAndInTheAction(t *testing.T) {
	page := gx.StringRequest(httptest.NewRequest("GET", "/cart", nil), menu())
	if !strings.Contains(page, `aria-current="page"`) || !strings.Contains(page, "data-gx-h=") {
		t.Fatalf("the fixture is wrong: the page is %s", page)
	}
	got, status := update(t, menu(), "/cart", pageHashes(menu(), "/cart"))
	if len(got) != 0 || status != http.StatusNoContent {
		t.Errorf("c.Update with the props of the page: patches of %v and status %d, want no patch and 204: no prop changed", got, status)
	}
}

// What the compiler writes for
//
//	<div><span #total>{total}</span>if note != "" { <p #note>{note}</p> }</div>
var (
	boxTmpl = gx.NewTemplate(
		[]string{"<div><span", ">", "</span>", "</div>"},
		[]int{1, 2, 1},
		[]gx.TemplateEl{{Slot: 0, Start: len("<div>"), EndStatic: 2, End: len("</span>")}},
		[]gx.TemplateRoot{{El: -1, Slot: -1, Name: "div"}},
	)
	noteTmpl = gx.NewTemplate(
		[]string{"<p", ">", "</p>"},
		[]int{0, 1},
		[]gx.TemplateEl{{Slot: 0, Start: 0, EndStatic: 2, End: len("</p>")}},
		[]gx.TemplateRoot{{El: 0, Slot: -1}},
	)
)

func box(total, note string) gx.Node {
	// The branch that the render does not take has the mark of its
	// fragment: the compiler writes gx.NoFragment there (B-019).
	var n gx.Node = gx.NoFragment("note")
	if note != "" {
		n = noteTmpl.With(gx.OpenFragment("p", idAttr("note")), gx.Text(note))
	}
	return boxTmpl.With(gx.OpenFragment("span", idAttr("total")), gx.Text(total), n)
}

// TestREQ_ACT_16_UpdateWithAFragmentThatIsGone checks REQ-ACT-16:
// "`c.Update(node)` takes the node of the invoking component from new
// props. The server compares the hash of each fragment with the hash of the
// request and sends each changed fragment as a patch through the adapter."
//
// The page shows the fragment #note, and the request holds its hash. With
// the new props the component has no #note. c.Update walks only the
// fragments of the new render, so the hash of the request with no fragment
// in the render is no difference: the answer has no patch and the status
// 204, and the page shows the note of before.
func TestREQ_ACT_16_UpdateWithAFragmentThatIsGone(t *testing.T) {
	have := pageHashes(box("10", "Free delivery"), "/cart")
	if !strings.Contains(have, "note=") || !strings.Contains(have, "total=") {
		t.Fatalf("the fixture is wrong: the page has the hashes %q", have)
	}
	got, status := update(t, box("10", ""), "/cart", have)
	if status == http.StatusForbidden {
		t.Fatalf("the fixture is wrong: status %d", status)
	}
	if len(got) == 0 {
		t.Errorf("c.Update with props that remove the fragment #note: no patch and status %d, want a patch that removes #note from the page", status)
	}
}
