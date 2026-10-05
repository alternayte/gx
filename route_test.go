package gx_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alternayte/gx"
)

type fakeRoute struct {
	pattern string
}

func (f fakeRoute) Pattern() string                              { return f.pattern }
func (f fakeRoute) ServeHTTP(http.ResponseWriter, *http.Request) {}

func TestREQ_RTE_07_DuplicateStartupPanic(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("duplicate route did not panic at startup")
		}
	}()
	app := gx.New(gx.Config{})
	app.Group("/", fakeRoute{"GET /x"}, fakeRoute{"GET /x"})
}

func TestREQ_RTE_08_Once(t *testing.T) {
	c := &gx.Ctx{}
	calls := 0
	f := func() (int, error) {
		calls++
		return 7, nil
	}
	get := func() int {
		v, err := gx.Once(c, f)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	if get() != 7 || get() != 7 {
		t.Fatalf("Once returned the wrong value")
	}
	if calls != 1 {
		t.Fatalf("fn ran %d times in one request, want 1", calls)
	}
	c2 := &gx.Ctx{}
	if _, err := gx.Once(c2, f); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("fn ran %d times across requests, want 2", calls)
	}
}

type slowRoute struct{}

func (slowRoute) Pattern() string          { return "GET /slow" }
func (slowRoute) Bind(*http.Request) error { return nil }

func TestREQ_RTE_08_ConcurrentLoaders(t *testing.T) {
	sleep := func() { time.Sleep(50 * time.Millisecond) }
	outer := gx.Layout(func(c *gx.Ctx) (int, error) { sleep(); return 1, nil },
		func(_ int, children gx.Node) gx.Node { return gx.El("main", nil, children) })
	inner := gx.Layout(func(c *gx.Ctx) (int, error) { sleep(); return 2, nil },
		func(_ int, children gx.Node) gx.Node { return gx.El("section", nil, children) })
	pg := gx.Page(func(c *gx.Ctx, in slowRoute) (int, error) { sleep(); return 3, nil },
		func(v int) gx.Node { return gx.Text("ok") })

	app := gx.New(gx.Config{})
	app.Group("/", outer, inner, gx.Collect(pg))
	start := time.Now()
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, httptest.NewRequest("GET", "/slow", nil))
	elapsed := time.Since(start)
	if rec.Code != 200 {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if got, want := rec.Body.String(), shell("<main><section>ok</section></main>"); got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
	if elapsed > 80*time.Millisecond {
		t.Fatalf("three 50ms loaders took %v, want under 80ms", elapsed)
	}
}

type homeRoute struct{}

func (homeRoute) URL() string { return "/home" }

func TestREQ_RTE_09_Middleware(t *testing.T) {
	var order []string
	mark := func(name string) func(http.Handler) http.Handler {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, name)
				next.ServeHTTP(w, r)
			})
		}
	}
	pg := gx.Page(func(c *gx.Ctx, in slowRoute) (int, error) { return 1, nil },
		func(int) gx.Node { return gx.Text("ok") })
	app := gx.New(gx.Config{})
	app.Group("/", mark("a"), mark("b"), gx.Collect(pg))
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, httptest.NewRequest("GET", "/slow", nil))
	if rec.Body.String() != shell("ok") {
		t.Fatalf("body = %q", rec.Body.String())
	}
	if len(order) != 2 || order[0] != "a" || order[1] != "b" {
		t.Fatalf("middleware order = %v, want [a b]", order)
	}
}

// shell returns the document shell the app writes around a page with no
// head, no stylesheet and no script.
func shell(body string) string {
	return `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"></head><body>` + body + `</body></html>`
}

func TestREQ_RTE_10_ErrorStatuses(t *testing.T) {
	serve := func(err error) *httptest.ResponseRecorder {
		pg := gx.Page(func(c *gx.Ctx, in slowRoute) (int, error) { return 0, err },
			func(int) gx.Node { return gx.Text("x") })
		app := gx.New(gx.Config{})
		app.Errors(func(c *gx.Ctx) gx.Node { return gx.Text("missing") }, nil, nil)
		app.Group("/", gx.Collect(pg))
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, httptest.NewRequest("GET", "/slow", nil))
		return rec
	}

	for _, c := range []struct {
		name   string
		err    error
		status int
		body   string
	}{
		{"404", gx.NotFound(), 404, shell("missing")},
		{"403", gx.Forbidden(), 403, "Forbidden\n"},
		{"500", errors.New("boom"), 500, "Internal Server Error\n"},
	} {
		rec := serve(c.err)
		if rec.Code != c.status {
			t.Fatalf("%s: status %d, want %d", c.name, rec.Code, c.status)
		}
		if rec.Body.String() != c.body {
			t.Fatalf("%s: body %q, want %q", c.name, rec.Body.String(), c.body)
		}
	}

	rec := serve(gx.Redirect(homeRoute{}))
	if rec.Code != 303 || rec.Header().Get("Location") != "/home" {
		t.Fatalf("redirect: status %d location %q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestREQ_RTE_11_Head(t *testing.T) {
	layout := gx.El("html", nil,
		gx.El("head", nil, gx.Head(gx.HeadProps{
			Title: "site",
			Meta:  []gx.Meta{{Name: "robots", Content: "index"}},
		})),
		gx.El("body", nil, gx.Frag(
			gx.Head(gx.HeadProps{
				Title: "page",
				Meta: []gx.Meta{
					{Name: "description", Content: "d"},
					{Name: "robots", Content: "noindex"},
				},
				Links: []gx.Link{{Rel: "canonical", Href: "/page"}},
			}),
			gx.Text("body"),
		)),
	)
	want := `<html><head><title>page</title><meta name="robots" content="noindex"><meta name="description" content="d"><link rel="canonical" href="/page"></head><body>body</body></html>`
	if got := gx.String(layout); got != want {
		t.Fatalf("head output = %q, want %q", got, want)
	}
}

func TestREQ_RTE_13_ActiveLinks(t *testing.T) {
	page := gx.El("nav", nil,
		gx.El("a", gx.Attrs{{Key: "href", Value: "/products/42", Kind: gx.AttrURL, Active: "page"}}, gx.Text("product")),
		gx.El("a", gx.Attrs{{Key: "href", Value: "/products", Kind: gx.AttrURL, Active: "section"}}, gx.Text("list")),
		gx.El("a", gx.Attrs{{Key: "href", Value: "/other", Kind: gx.AttrURL, Active: "page"}}, gx.Text("other")),
	)
	req := httptest.NewRequest("GET", "/products/42", nil)
	got := gx.StringRequest(req, page)
	want := `<nav><a href="/products/42" data-gx-active="page" aria-current="page">product</a><a href="/products" data-gx-active="section" data-active>list</a><a href="/other" data-gx-active="page">other</a></nav>`
	if got != want {
		t.Fatalf("active links = %q, want %q", got, want)
	}
	if got := gx.String(page); got == want {
		t.Fatal("no request in scope must not mark links")
	}
}

type staticRoute struct {
	ID int64
}

func (staticRoute) Pattern() string          { return "GET /products/{id}" }
func (staticRoute) Bind(*http.Request) error { return nil }

func TestREQ_RTE_15_Static(t *testing.T) {
	pg := gx.Page(func(c *gx.Ctx, in staticRoute) (int64, error) { return in.ID, nil },
		func(v int64) gx.Node { return gx.Value(v) })
	pg.Static(func() ([]staticRoute, error) {
		return []staticRoute{{ID: 1}, {ID: 2}}, nil
	})
	ins, ok, err := gx.StaticInputs(pg)
	if err != nil || !ok || len(ins) != 2 {
		t.Fatalf("StaticInputs = %v, %v, %v", ins, ok, err)
	}
	if ins[1].(staticRoute).ID != 2 {
		t.Fatalf("second input = %+v", ins[1])
	}

	plain := gx.Page(func(c *gx.Ctx, in staticRoute) (int64, error) { return in.ID, nil },
		func(v int64) gx.Node { return gx.Value(v) })
	if _, ok, _ := gx.StaticInputs(plain); ok {
		t.Fatal("a page without Static reports inputs")
	}
}

func TestREQ_RTE_16_RenderInHandler(t *testing.T) {
	page := gx.El("nav", nil,
		gx.El("a", gx.Attrs{{Key: "href", Value: "/here", Kind: gx.AttrURL, Active: "page"}}, gx.Text("here")),
	)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := gx.Render(w, r, page); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/here", nil))
	if rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Fatalf("content type = %q", ct)
	}
	want := `<nav><a href="/here" data-gx-active="page" aria-current="page">here</a></nav>`
	if got := rec.Body.String(); got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}

type paramRoute struct {
	ID string
}

func (paramRoute) Pattern() string { return "GET /products/{id}" }
func (in *paramRoute) Bind(r *http.Request) error {
	in.ID = gx.PathValue(r, "id")
	return nil
}

func TestREQ_RTE_17_Params(t *testing.T) {
	pg := gx.Page(func(c *gx.Ctx, in paramRoute) (string, error) { return in.ID, nil },
		func(s string) gx.Node { return gx.Text(s) })

	// Registered one by one on the standard ServeMux.
	mux := http.NewServeMux()
	mux.Handle(pg.Pattern(), pg)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/products/42", nil))
	if rec.Body.String() != "42" {
		t.Fatalf("ServeMux body = %q", rec.Body.String())
	}

	// A router that does not fill PathValue uses gx.Params.
	h := gx.Params(func(r *http.Request, name string) string {
		if name == "id" {
			return "77"
		}
		return ""
	})(pg)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/products/ignored", nil))
	if rec.Body.String() != "77" {
		t.Fatalf("Params body = %q", rec.Body.String())
	}
}

func TestREQ_RTE_18_Mount(t *testing.T) {
	pg := gx.Page(func(c *gx.Ctx, in slowRoute) (int, error) { return 1, nil },
		func(int) gx.Node { return gx.Text("ok") })
	app := gx.New(gx.Config{BasePath: "/shop"})
	defer gx.SetBasePath("")
	app.Group("/", gx.Collect(pg))

	mux := http.NewServeMux()
	mux.Handle("/shop/", http.StripPrefix("/shop", app))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/shop/slow", nil))
	if rec.Code != 200 || rec.Body.String() != shell("ok") {
		t.Fatalf("mounted app: %d %q", rec.Code, rec.Body.String())
	}
	if gx.BasePath() != "/shop" {
		t.Fatalf("BasePath = %q", gx.BasePath())
	}
}

func TestREQ_RTE_19_NoSpecialMiddleware(t *testing.T) {
	var order []string
	std := func(name string) func(http.Handler) http.Handler {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, name+":before")
				next.ServeHTTP(w, r)
				order = append(order, name+":after")
			})
		}
	}

	pg := gx.Page(func(c *gx.Ctx, in slowRoute) (int, error) { return 1, nil },
		func(int) gx.Node { return gx.Text("ok") })
	app := gx.New(gx.Config{})
	app.Group("/", gx.Collect(pg))
	h := std("app")(app)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/slow", nil))
	if rec.Body.String() != shell("ok") {
		t.Fatalf("app body = %q", rec.Body.String())
	}

	node := gx.Text("plain")
	h2 := std("render")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := gx.Render(w, r, node); err != nil {
			t.Fatal(err)
		}
	}))
	rec = httptest.NewRecorder()
	h2.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Body.String() != "plain" {
		t.Fatalf("render body = %q", rec.Body.String())
	}

	want := []string{"app:before", "app:after", "render:before", "render:after"}
	if len(order) != len(want) {
		t.Fatalf("middleware order = %v", order)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("middleware order = %v, want %v", order, want)
		}
	}
}
