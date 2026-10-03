package gx_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alternayte/gx"
)

type navPage struct{}

func (navPage) Pattern() string          { return "GET /a" }
func (navPage) Bind(*http.Request) error { return nil }

// TestREQ_RTE_12_LayoutFallback checks that a request whose layout chain the
// client does not share falls back to a full load (REQ-RTE-12).
func TestREQ_RTE_12_LayoutFallback(t *testing.T) {
	layout := gx.Layout(
		func(c *gx.Ctx) (struct{}, error) { return struct{}{}, nil },
		func(_ struct{}, children gx.Node) gx.Node {
			return gx.El("main", nil, children)
		})
	pg := gx.Page(func(c *gx.Ctx, in navPage) (int, error) { return 1, nil },
		func(int) gx.Node { return gx.El("h1", nil, gx.Text("a")) })
	a := &fakeAdapter{}
	app := gx.New(gx.Config{Adapter: a})
	app.Group("/", layout, gx.Nav(gx.MorphNavigation), gx.Collect(pg))

	// A plain request renders the full page with a layout slot.
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, httptest.NewRequest("GET", "/a", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "data-gx-slot=") {
		t.Fatalf("full page = %d %q", rec.Code, rec.Body.String())
	}

	// A navigation request with no shared layout answers a full load.
	req := httptest.NewRequest("GET", "/a", nil)
	req.Header.Set("Gx-Nav", "1")
	req.Header.Set("Gx-Layouts", "other.go:1")
	rec = httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != 200 || rec.Header().Get("Gx-Nav") != "full" {
		t.Fatalf("fallback = %d header %q, want Gx-Nav: full", rec.Code, rec.Header().Get("Gx-Nav"))
	}
}
