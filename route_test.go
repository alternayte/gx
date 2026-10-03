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
	if got, want := rec.Body.String(), "<main><section>ok</section></main>"; got != want {
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
	if rec.Body.String() != "ok" {
		t.Fatalf("body = %q", rec.Body.String())
	}
	if len(order) != 2 || order[0] != "a" || order[1] != "b" {
		t.Fatalf("middleware order = %v, want [a b]", order)
	}
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
		{"404", gx.NotFound(), 404, "missing"},
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
