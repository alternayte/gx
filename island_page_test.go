package gx_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	gx "github.com/alternayte/gx"
)

// A page with an island loads the island loader and no other script. The
// loader needs no adapter (REQ-ISL-04, NFR-04).
func TestREQ_ISL_04_PageWithIslandLoadsTheLoader(t *testing.T) {
	view := func() gx.Node {
		return gx.El("main", nil, gx.Island("app/dash/Chart", `{}`))
	}
	for name, adapter := range map[string]gx.Adapter{"no adapter": nil, "adapter": &nfr04Adapter{}} {
		body := servePage(t, adapter, view)
		if !strings.Contains(body, `<script type="module" src="/_gx/island.js"></script>`) {
			t.Fatalf("%s: the page does not load the island loader:\n%s", name, body)
		}
		if n := strings.Count(body, "<script"); n != 1 {
			t.Fatalf("%s: the page has %d scripts, want the loader only:\n%s", name, n, body)
		}
	}
	plain := servePage(t, &nfr04Adapter{}, func() gx.Node { return gx.El("main", nil, gx.Text("plain")) })
	if strings.Contains(plain, "<script") {
		t.Fatalf("a page with no island has a script:\n%s", plain)
	}

	app := gx.New(gx.Config{})
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, httptest.NewRequest("GET", "/_gx/island.js", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "gx-island") {
		t.Fatalf("GET /_gx/island.js = %d", rec.Code)
	}
}

func TestREQ_ISL_04_SignalRefPropIsAListOfPathParts(t *testing.T) {
	ref := gx.Ref[int](gx.SignalRefPath("cart.Cart", gx.Key("42"), "qty"))
	if got := string(gx.AppendJSONSignalRef(nil, string(ref))); got != `{"$signal":["cart","Cart","42","qty"]}` {
		t.Fatalf("signal ref = %s", got)
	}
	if got := string(gx.AppendJSONSignalRef(nil, "")); got != `{"$signal":[]}` {
		t.Fatalf("empty ref = %s", got)
	}
}
