package gx_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alternayte/gx"
)

// TestREQ_RTE_21_ETag checks the tag of a GET page: the answer has an ETag
// of its bytes, a request with the same tag gets 304 with no body, and a
// page with different bytes gets a different tag. The loader runs for each
// request (REQ-RTE-21).
func TestREQ_RTE_21_ETag(t *testing.T) {
	text, loads := "one", 0
	cacheControl := ""
	pg := gx.Page(func(c *gx.Ctx, in slowRoute) (string, error) {
		loads++
		if cacheControl != "" {
			c.W.Header().Set("Cache-Control", cacheControl)
		}
		return text, nil
	}, func(s string) gx.Node { return gx.El("p", nil, gx.Text(s)) })
	act := gx.Action(func(c *gx.Ctx, in actRoute) error { return nil })
	app := gx.New(gx.Config{Adapter: &fakeAdapter{}})
	app.Group("/", gx.Collect(pg, act))
	get := func(h http.Handler, header ...string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", "/slow", nil)
		for i := 0; i+1 < len(header); i += 2 {
			req.Header.Set(header[i], header[i+1])
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	first := get(app)
	tag := first.Header().Get("ETag")
	if first.Code != 200 || tag == "" || first.Header().Get("Cache-Control") != "private, no-cache" {
		t.Fatalf("first answer: status %d, ETag %q, Cache-Control %q", first.Code, tag, first.Header().Get("Cache-Control"))
	}
	// The same tag: 304, no body, and the loader ran.
	second := get(app, "If-None-Match", tag)
	if second.Code != http.StatusNotModified || second.Body.Len() != 0 || second.Header().Get("ETag") != tag {
		t.Errorf("same tag: status %d, body %q, ETag %q, want 304 with no body", second.Code, second.Body.String(), second.Header().Get("ETag"))
	}
	if loads != 2 {
		t.Errorf("the loader ran %d times for 2 requests", loads)
	}
	// A list of tags, as a cache sends it.
	if rec := get(app, "If-None-Match", `"other", `+tag); rec.Code != http.StatusNotModified {
		t.Errorf("a list with the tag: status %d, want 304", rec.Code)
	}
	if rec := get(app, "If-None-Match", `"other"`); rec.Code != 200 || rec.Body.Len() == 0 {
		t.Errorf("a different tag: status %d, want 200 with the page", rec.Code)
	}
	// Different bytes: a different tag, and the old tag gets the page.
	text = "two"
	third := get(app, "If-None-Match", tag)
	if third.Code != 200 || third.Header().Get("ETag") == tag || third.Header().Get("ETag") == "" {
		t.Errorf("a changed page: status %d, ETag %q (was %q)", third.Code, third.Header().Get("ETag"), tag)
	}

	// A header of the app stays.
	cacheControl = "max-age=60"
	if rec := get(app); rec.Header().Get("Cache-Control") != "max-age=60" || rec.Header().Get("ETag") == "" {
		t.Errorf("a Cache-Control of the app: %q with ETag %q, want the header of the app and a tag", rec.Header().Get("Cache-Control"), rec.Header().Get("ETag"))
	}
	cacheControl = ""

	// No tag: a page with a CSP nonce, a partial navigation, an answer of
	// an action and a request that is not a GET.
	if rec := get(gx.CSP(gx.CSPOptions{})(app)); rec.Code != 200 || rec.Header().Get("ETag") != "" {
		t.Errorf("a page with a nonce: status %d, ETag %q, want no tag", rec.Code, rec.Header().Get("ETag"))
	}
	if rec := get(gx.CSP(gx.CSPOptions{})(app), "If-None-Match", tag); rec.Code != 200 {
		t.Errorf("a page with a nonce and a tag in the request: status %d, want 200", rec.Code)
	}
	if rec := get(app, "Gx-Nav", "1"); rec.Header().Get("ETag") != "" {
		t.Errorf("a partial navigation has the ETag %q", rec.Header().Get("ETag"))
	}
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, httptest.NewRequest("POST", "/act", nil))
	if rec.Header().Get("ETag") != "" {
		t.Errorf("the answer of an action has the ETag %q", rec.Header().Get("ETag"))
	}
}

// TestREQ_RTE_21_NoTagForAWidgetOrAnError checks that the answer of a widget
// and a page with an error status have no tag (REQ-RTE-21).
func TestREQ_RTE_21_NoTagForAWidgetOrAnError(t *testing.T) {
	missing := gx.Page(func(c *gx.Ctx, in slowRoute) (int, error) { return 0, gx.NotFound() },
		func(int) gx.Node { return gx.Text("x") })
	app := gx.New(gx.Config{})
	app.Errors(func(c *gx.Ctx) gx.Node { return gx.Text("missing") }, nil, nil)
	app.Group("/", gx.Collect(missing))
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, httptest.NewRequest("GET", "/slow", nil))
	if rec.Code != 404 || rec.Header().Get("ETag") != "" {
		t.Errorf("an error page: status %d, ETag %q, want 404 with no tag", rec.Code, rec.Header().Get("ETag"))
	}
	req := httptest.NewRequest("GET", "/slow", nil)
	req.Header.Set("Gx-Widget", "shop-cart")
	rec = httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Header().Get("ETag") != "" {
		t.Errorf("a widget request has the ETag %q", rec.Header().Get("ETag"))
	}
}
