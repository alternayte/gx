package round2_test

import (
	"path/filepath"
	"testing"
)

// contentApp is a content site in the shape of SDD §12.4: one collection
// with typed frontmatter, one component and one page per entry.
var contentApp = map[string]string{
	"docs/Aside.gx": "package docs\n\nprops {\n  Children gx.Node\n}\n\n<aside class=\"gx-aside\">{p.Children}</aside>\n",
	"docs/content.go": `package docs

import (
	"github.com/alternayte/gx"
	"github.com/alternayte/gx/content"
)

type DocMeta struct {
	Title string ` + "`yaml:\"title\"`" + `
}

var Docs = gx.Collection[DocMeta]("content/docs").Components(Aside)

func View(e gx.Entry[DocMeta]) gx.Node {
	return gx.El("main", nil, gx.El("h1", nil, gx.Text(e.Meta.Title)), DocsBody(e))
}

var Pages = gx.ContentEntries(Docs, View)

func init() { content.Install() }
`,
	"content/docs/start.md": "---\ntitle: Your first stream\n---\n\nintro text\n\n<docs.Aside>Tip body</docs.Aside>\n",
	"docs/serve_test.go": `package docs

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// TestInnerContentPage serves the content page with the working directory
// that R2_CWD names.
func TestInnerContentPage(t *testing.T) {
	if err := os.Chdir(os.Getenv("R2_CWD")); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("GET /{slug...}", Pages)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/start", nil))
	body := rec.Body.String()
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /start = %d, want 200", rec.Code)
	}
	for _, want := range []string{"Your first stream", "intro text", "Tip body"} {
		if !strings.Contains(body, want) {
			t.Fatalf("the page lacks %q:\n%s", want, body)
		}
	}
}
`,
}

// TestNFR_08_ContentPagesAreInTheBinary: NFR-08 says the production app is
// one binary with all assets embedded, and §12.4 and REQ-CNT-01 say that
// Markdown compiles at build time into Go code. The generated
// gxcontent_gx.go holds the body text, but gx.Collection reads the entry
// list and the frontmatter from the content directory on disk at request
// time (content.go, loadEntries). A production binary that runs without
// the source tree answers 404 for every content page.
//
// The test builds the packages without the gxdev tag. The first run has
// the module root as its working directory and proves that the app is
// correct. The second run has an empty working directory, as a deployed
// binary has.
func TestNFR_08_ContentPagesAreInTheBinary(t *testing.T) {
	// The content package needs the YAML and Markdown modules of gx.
	t.Setenv("GOFLAGS", "-mod=mod")
	dir := scratchModule(t, contentApp)
	generateInto(t, dir)
	root, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	if out := goTest(dir, "TestInnerContentPage", "R2_CWD="+root); !innerPassed(out, "TestInnerContentPage") {
		t.Fatalf("the content app does not serve its page from the source tree; the test app is wrong:\n%s", tail(out, 15))
	}
	if out := goTest(dir, "TestInnerContentPage", "R2_CWD="+t.TempDir()); !innerPassed(out, "TestInnerContentPage") {
		t.Fatalf("the content page needs the Markdown files beside the binary:\n%s", tail(out, 8))
	}
}
