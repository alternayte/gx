package gx_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alternayte/gx"
)

// fragmentPage renders a layout the way an app does: a fragment with a
// gx.Head, and no html, head or body tag.
type fragmentPage struct{ node func() gx.Node }

func (fragmentPage) Pattern() string { return "/fragment" }

func (p fragmentPage) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = gx.RenderRequest(w, r, p.node())
}

// serveFragment returns the response body of the fragment page.
func serveFragment(t *testing.T, node func() gx.Node) string {
	t.Helper()
	app := gx.New(gx.Config{})
	app.Group("/", fragmentPage{node: node})
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, httptest.NewRequest("GET", "/fragment", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	return rec.Body.String()
}

// TestREQ_RTE_11_DocumentShell covers the document shell: a full-page
// response is a complete document, and the stylesheet link and the gx.Head
// output sit in the head, before any body content. A stylesheet link after
// the content paints the page unstyled first.
func TestREQ_RTE_11_DocumentShell(t *testing.T) {
	gx.SetStylesheet([]byte(".gx-test { color: red }"))
	defer gx.SetStylesheet(nil)

	body := serveFragment(t, func() gx.Node {
		return gx.Frag(
			gx.El("header", nil, gx.Text("site")),
			gx.Head(gx.HeadProps{Title: "Page", Meta: []gx.Meta{{Name: "description", Content: "d"}}}),
			gx.El("main", nil, gx.Text("content")),
		)
	})
	if !strings.HasPrefix(body, "<!doctype html><html lang=\"en\"><head>") {
		t.Fatalf("the page is not a document:\n%s", body)
	}
	headEnd := strings.Index(body, "</head><body>")
	if headEnd < 0 {
		t.Fatalf("the page has no head and body:\n%s", body)
	}
	head, rest := body[:headEnd], body[headEnd:]
	for _, want := range []string{
		`<meta charset="utf-8">`,
		`<meta name="viewport" content="width=device-width, initial-scale=1">`,
		`<title>Page</title>`,
		`<meta name="description" content="d">`,
		`<link rel="stylesheet" href="/_gx/app.css">`,
	} {
		if !strings.Contains(head, want) {
			t.Errorf("the head lacks %s:\n%s", want, head)
		}
	}
	for _, stray := range []string{"<title>", "<meta", "<link"} {
		if strings.Contains(rest, stray) {
			t.Errorf("the body holds %s:\n%s", stray, rest)
		}
	}
	if !strings.HasSuffix(body, "<header>site</header><main>content</main></body></html>") {
		t.Errorf("the body content changed:\n%s", rest)
	}
}
