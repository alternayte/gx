package gx_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alternayte/gx"
)

// styledPage is a plain handler; gx.Route types need generated methods.
type styledPage struct{}

func (styledPage) Pattern() string { return "/styled" }

func (styledPage) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte("<html><head></head><body>hi</body></html>"))
}

// TestREQ_STY_03_StylesheetRoute covers the stylesheet contract: the app
// serves the installed CSS at /_gx/app.css and links it in the page head
// (REQ-STY-03).
func TestREQ_STY_03_StylesheetRoute(t *testing.T) {
	css := []byte(".gx-test { color: red }")
	gx.SetStylesheet(css)
	defer gx.SetStylesheet(nil)

	app := gx.New(gx.Config{})
	app.Group("/", styledPage{})

	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, httptest.NewRequest("GET", "/_gx/app.css", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("css status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/css") {
		t.Fatalf("css content type = %q", ct)
	}
	if rec.Body.String() != string(css) {
		t.Fatalf("css body = %q", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	app.ServeHTTP(rec, httptest.NewRequest("GET", "/styled", nil))
	if !strings.Contains(rec.Body.String(), `<link rel="stylesheet" href="/_gx/app.css">`) {
		t.Fatalf("page lacks the stylesheet link:\n%s", rec.Body.String())
	}

	// Without a stylesheet the route is absent and no link is added.
	gx.SetStylesheet(nil)
	rec = httptest.NewRecorder()
	app.ServeHTTP(rec, httptest.NewRequest("GET", "/_gx/app.css", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("empty stylesheet status = %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	app.ServeHTTP(rec, httptest.NewRequest("GET", "/styled", nil))
	if strings.Contains(rec.Body.String(), "_gx/app.css") {
		t.Fatal("page links a missing stylesheet")
	}
}
