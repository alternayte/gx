package round8_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/alternayte/gx"
	"github.com/alternayte/gx/tests/review/round-8/fix/box"
)

// boxSource is the component of fix/box. The file box_generated.go there
// is what the compiler writes for it; the test checks that.
const boxSource = `package box

props {
  Total string
  Note  string
}

<div id="box">
  <span #total>{p.Total}</span>
  if p.Note != "" {
    <p #note>{p.Note}</p>
  }
</div>
`

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

var (
	idRE   = regexp.MustCompile(` id="([^"]+)"`)
	hashRE = regexp.MustCompile(`id="([^"]+)" data-gx-h="([0-9a-f]{8})"`)
)

// TestREQ_ACT_16_UpdateWithAFragmentThatIsNew checks REQ-ACT-16:
// "`c.Update(node)` takes the node of the invoking component from new
// props. The server compares the hash of each fragment with the hash of the
// request and sends each changed fragment as a patch through the adapter."
//
// The page shows the box with no note, so the page has no element #box-note
// and the request has no hash of it. With the new props the box has the
// fragment #note. c.Update sends it as a morph patch with the target
// #box-note. The page has no such element: the Datastar adapter then fails
// with PatchElementsNoTargetsFound, and htmx drops the element. The note of
// the new props never shows. Round 1 fixed the other direction (a fragment
// that the new render does not have) with gx.NoFragment.
//
// The test takes each patch of the answer and requires that the page has
// the element of its target.
func TestREQ_ACT_16_UpdateWithAFragmentThatIsNew(t *testing.T) {
	// The fixture is what the compiler writes.
	dir := scratchModule(t, map[string]string{"ui/box/Box.gx": boxSource})
	generated := generateModule(t, dir)[filepath.Join(dir, "ui", "box", "Box_gx.go")]
	onDisk, err := os.ReadFile(filepath.Join(repoRoot(t), "tests", "review", "round-8", "fix", "box", "box_generated.go"))
	if err != nil || !bytes.Equal(generated, onDisk) {
		t.Fatalf("fix/box/box_generated.go is not what the compiler writes for the component (%v):\n%s", err, generated)
	}

	// The page of GET /cart: the box with no note.
	page := gx.StringRequest(httptest.NewRequest("GET", "/cart", nil), box.Box(box.BoxProps{Total: "10"}))
	onPage := map[string]bool{}
	for _, m := range idRE.FindAllStringSubmatch(page, -1) {
		onPage["#"+m[1]] = true
	}
	var hashes []string
	for _, m := range hashRE.FindAllStringSubmatch(page, -1) {
		hashes = append(hashes, m[1]+"="+m[2])
	}
	if !onPage["#box"] || !onPage["#box-total"] || onPage["#box-note"] || len(hashes) != 1 {
		t.Fatalf("the fixture is wrong: the page is %s", page)
	}

	// The action gives the box a note.
	a := &recAdapter{}
	app := gx.New(gx.Config{Adapter: a})
	app.Group("/", gx.Collect(gx.Action(func(c *gx.Ctx, in actIn) error {
		return c.Update(box.Box(box.BoxProps{Total: "10", Note: "Free delivery"}))
	})))
	req := httptest.NewRequest("POST", "/act", nil)
	req.Header.Set("Referer", "http://example.com/cart")
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Gx-Fragments", strings.Join(hashes, ","))
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if a.res == nil {
		t.Fatalf("the action gave no response: status %d", rec.Code)
	}
	if len(a.res.Patches) == 0 {
		t.Fatalf("c.Update with props that add the fragment #note: no patch, status %d", rec.Code)
	}
	for _, p := range a.res.Patches {
		ep, ok := p.(gx.ElementPatch)
		if !ok {
			continue
		}
		if !onPage[ep.Target] {
			t.Errorf("c.Update sends a patch with the target %s (mode %d), and the page has no such element; the page has %s. The note of the new props does not show.", ep.Target, ep.Mode, page)
		}
	}
}
