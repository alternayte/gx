package round8_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alternayte/gx"
)

type docRoute struct{}

func (docRoute) Pattern() string          { return "GET /doc" }
func (docRoute) Bind(*http.Request) error { return nil }

// TestREQ_RTE_21_ETagOfTheAppStays checks the acceptance of REQ-RTE-21: "A
// header of the app stays."
//
// The loader of the page sets the ETag of its document: the version of the
// row that the page shows. The answer has a different ETag, of the bytes of
// the page: Gx replaces the header of the app. A request with the tag of
// the app in If-None-Match then gets no 304, and a proxy that keys on the
// tag of the app sees a tag that the app did not set. The Cache-Control of
// the app stays; the ETag of the app does not.
func TestREQ_RTE_21_ETagOfTheAppStays(t *testing.T) {
	const version = `"doc-7-v42"`
	pg := gx.Page(func(c *gx.Ctx, in docRoute) (string, error) {
		c.W.Header().Set("ETag", version)
		return "The document", nil
	}, func(s string) gx.Node { return gx.El("p", nil, gx.Text(s)) })
	app := gx.New(gx.Config{})
	app.Group("/", gx.Collect(pg))
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, httptest.NewRequest("GET", "/doc", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("the fixture is wrong: status %d", rec.Code)
	}
	if got := rec.Header().Get("ETag"); got != version {
		t.Errorf("the loader set the ETag %s, and the answer has the ETag %s: the header of the app does not stay", version, got)
	}
}
