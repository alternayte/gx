package gx_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alternayte/gx"
)

// TestREQ_STY_13_RouteStylesheet checks the stylesheet of a page route at
// run time: the page links the file of its route by a name with the hash of
// the content, two routes with the same content share one file, each other
// page links the stylesheet of the app, and the app serves the file for a
// long time (REQ-STY-13).
func TestREQ_STY_13_RouteStylesheet(t *testing.T) {
	gx.SetStylesheet([]byte(".all{color:red}.one{color:blue}"))
	gx.SetRouteStylesheets(map[string][]byte{
		"GET /nfr04": []byte(".one{color:blue}"),
		"GET /twin":  []byte(".one{color:blue}"),
		"GET /empty": nil,
	})
	t.Cleanup(func() { gx.SetStylesheet(nil); gx.SetRouteStylesheets(nil) })
	page := func(pattern string) gx.Handler {
		return fakePage{pattern: pattern}
	}
	app := gx.New(gx.Config{})
	app.Group("/", page("GET /nfr04"), page("GET /twin"), page("GET /home"), page("GET /empty"))
	get := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		return rec
	}
	href := func(body string) string {
		_, rest, ok := strings.Cut(body, `<link rel="stylesheet" href="`)
		if !ok {
			return ""
		}
		url, _, _ := strings.Cut(rest, `"`)
		return url
	}

	own := href(get("/nfr04").Body.String())
	if !strings.HasPrefix(own, "/_gx/css/app.") || !strings.HasSuffix(own, ".css") {
		t.Fatalf("the page of a route with a stylesheet links %q", own)
	}
	if twin := href(get("/twin").Body.String()); twin != own {
		t.Errorf("two routes with the same content link %q and %q, want one file", own, twin)
	}
	for _, path := range []string{"/home", "/empty"} {
		if got := href(get(path).Body.String()); got != "/_gx/app.css" {
			t.Errorf("%s links %q, want the stylesheet of the app", path, got)
		}
	}
	// The Link header names the same file (NFR-04).
	if link := get("/nfr04").Header().Get("Link"); link != "<"+own+">; rel=preload; as=style" {
		t.Errorf("Link = %q, want the stylesheet of the route", link)
	}
	file := get(own)
	if file.Code != 200 || file.Body.String() != ".one{color:blue}" || !strings.Contains(file.Header().Get("Cache-Control"), "immutable") {
		t.Errorf("the file: status %d, body %q, Cache-Control %q", file.Code, file.Body.String(), file.Header().Get("Cache-Control"))
	}
	if rec := get("/_gx/css/app.000000000000.css"); rec.Code != 404 {
		t.Errorf("a name with no file: status %d, want 404", rec.Code)
	}
}

// fakePage is a hand-written page route with one paragraph.
type fakePage struct{ pattern string }

func (f fakePage) Pattern() string { return f.pattern }

func (f fakePage) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_ = gx.Render(w, r, gx.El("p", nil, gx.Text("page")))
}

// TestREQ_STY_13_NavigationNamesTheStylesheet checks that the answer of a
// partial navigation names the stylesheet of its page, so the runtime links
// it before it applies the patch (REQ-STY-13, REQ-RTE-12).
func TestREQ_STY_13_NavigationNamesTheStylesheet(t *testing.T) {
	gx.SetStylesheet([]byte(".all{color:red}"))
	gx.SetRouteStylesheets(map[string][]byte{"GET /slow": []byte(".one{color:blue}")})
	t.Cleanup(func() { gx.SetStylesheet(nil); gx.SetRouteStylesheets(nil) })
	layout := gx.Layout(func(c *gx.Ctx) (struct{}, error) { return struct{}{}, nil },
		func(_ struct{}, children gx.Node) gx.Node { return gx.El("main", nil, children) })
	pg := gx.Page(func(c *gx.Ctx, in slowRoute) (int, error) { return 1, nil },
		func(int) gx.Node { return gx.El("p", nil, gx.Text("slow")) })
	a := &fakeAdapter{}
	app := gx.New(gx.Config{Adapter: a})
	app.Group("/", layout, gx.Nav(gx.MorphNavigation), gx.Collect(pg))

	page := httptest.NewRecorder()
	app.ServeHTTP(page, httptest.NewRequest("GET", "/slow", nil))
	_, rest, _ := strings.Cut(page.Body.String(), `<link rel="stylesheet" href="`)
	own, _, _ := strings.Cut(rest, `"`)
	_, rest, _ = strings.Cut(page.Body.String(), `data-gx-slot="`)
	id, _, _ := strings.Cut(rest, `"`)

	req := httptest.NewRequest("GET", "/slow", nil)
	req.Header.Set("Gx-Nav", "1")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Gx-Layouts", id)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if !a.responded || rec.Header().Get("Gx-Nav") == "full" {
		t.Fatalf("the navigation is not a patch: Gx-Nav %q", rec.Header().Get("Gx-Nav"))
	}
	if got := rec.Header().Get("Gx-Sheet"); got != own || !strings.HasPrefix(own, "/_gx/css/app.") {
		t.Errorf("Gx-Sheet = %q, want the stylesheet of the page %q", got, own)
	}
}
