package gx_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alternayte/gx"
)

// otherAdapter is a second adapter with its own invocation syntax.
type otherAdapter struct{ fakeAdapter }

func (*otherAdapter) Invoke(method, url, scope string) gx.Attr {
	return gx.Attr{Key: "data-other-on", Value: "other(" + url + ")"}
}

// invokeButton is what a component builds: it has no request in scope.
func invokeButton() gx.Node {
	return gx.Frag(
		gx.El("button", gx.Attrs{{Key: "data-on:click", Value: "$n = 1; " + gx.Invoke("POST", "/act", "cart.Cart").Value}}, gx.Text("Add")),
		gx.El("button", gx.Attrs{{Key: "type", Value: "button"}, gx.Invoke("POST", "/undo", "")}, gx.Text("Undo")),
	)
}

// TestREQ_PLG_04_InvokeUsesTheRequestAdapter checks that the adapter of the
// app that serves the request writes the invocation, not the process
// default.
func TestREQ_PLG_04_InvokeUsesTheRequestAdapter(t *testing.T) {
	old := gx.AdapterOf(nil)
	defer gx.SetAdapter(old)
	page := gx.Page(func(*gx.Ctx, nfr04Route) (struct{}, error) { return struct{}{}, nil },
		func(struct{}) gx.Node { return invokeButton() })
	app := gx.New(gx.Config{Adapter: &fakeAdapter{}})
	app.Group("/", gx.Collect(page))
	gx.SetAdapter(&otherAdapter{})
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, httptest.NewRequest("GET", "/nfr04", nil))
	body := rec.Body.String()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %q", rec.Code, body)
	}
	for _, want := range []string{
		`<button data-on:click="$n = 1; fake(POST /act cart.Cart)">Add</button>`,
		`<button type="button" data-fake-on="fake(POST /undo )">Undo</button>`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("page lacks %s:\n%s", want, body)
		}
	}
}

// TestREQ_PLG_04_InvokeUsesTheProcessDefault checks a render with no
// request, as in a test or a fixture.
func TestREQ_PLG_04_InvokeUsesTheProcessDefault(t *testing.T) {
	old := gx.AdapterOf(nil)
	defer gx.SetAdapter(old)
	gx.SetAdapter(&otherAdapter{})
	got := gx.String(invokeButton())
	want := `<button data-on:click="$n = 1; other(/act)">Add</button><button type="button" data-other-on="other(/undo)">Undo</button>`
	if got != want {
		t.Fatalf("render:\n got %s\nwant %s", got, want)
	}
}

// TestREQ_PLG_04_InvokeWithNoAdapter checks that a render with no adapter
// writes no invocation and no placeholder.
func TestREQ_PLG_04_InvokeWithNoAdapter(t *testing.T) {
	old := gx.AdapterOf(nil)
	defer gx.SetAdapter(old)
	gx.SetAdapter(nil)
	got := gx.String(invokeButton())
	want := `<button data-on:click="$n = 1; ">Add</button><button type="button">Undo</button>`
	if got != want {
		t.Fatalf("render:\n got %q\nwant %q", got, want)
	}
}

// TestREQ_PLG_04_InvokeLoadsTheAdapter checks that a page whose only client
// feature is an invocation attribute ships the adapter runtime (NFR-04).
func TestREQ_PLG_04_InvokeLoadsTheAdapter(t *testing.T) {
	body := servePage(t, &nfr04Adapter{}, func() gx.Node {
		return gx.El("html", nil, gx.El("head", nil), gx.El("body", nil,
			gx.El("button", gx.Attrs{gx.Invoke("POST", "/undo", "")}, gx.Text("Undo"))))
	})
	if !strings.Contains(body, "/_gx/nfr04-adapter.js") {
		t.Fatalf("page lacks the adapter runtime:\n%s", body)
	}
}
