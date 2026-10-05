package site_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/gx"
	"github.com/alternayte/gx/adapters/datastar"
	"github.com/alternayte/gx/content"
	"github.com/alternayte/gx/docs/site"
)

// TestREQ_DOC_03_DiagnosticRoutes covers the address of a diagnostic page:
// the doc link of a diagnostic is /errors/<code>, and the docs site serves
// that page with its cause, its example and its fix. The index links to
// every page (REQ-DOC-03).
func TestREQ_DOC_03_DiagnosticRoutes(t *testing.T) {
	t.Chdir("..") // the collection reads content/ under the module root
	content.Install()
	app := gx.New(gx.Config{Adapter: datastar.Adapter()})
	app.Group("/", site.Routes)

	files, err := filepath.Glob(filepath.Join("content", "errors", "GX*.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 40 {
		t.Fatalf("content/errors holds %d pages", len(files))
	}
	index := get(t, app, "/reference/diagnostics/")
	for _, file := range files {
		code := strings.TrimSuffix(filepath.Base(file), ".md")
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(raw), "code: "+code) {
			t.Errorf("%s does not name its code", file)
		}
		href := "/errors/" + code + "/"
		if !strings.Contains(index, `href="`+href+`"`) {
			t.Errorf("the index does not link to %s", href)
		}
		page := get(t, app, href)
		text := tags.ReplaceAllString(page, "|")
		for _, want := range []string{code + ":", "|Cause|", "|Example|", "|Fix|"} {
			if !strings.Contains(text, want) {
				t.Errorf("%s lacks %q", href, want)
			}
		}
		// The example is a code frame with the file name as its title.
		if !strings.Contains(page, `class="gx-code"`) || !strings.Contains(page, `class="gx-code-title"`) {
			t.Errorf("%s has no titled code frame", href)
		}
		// The sidebar holds the page in the Diagnostics section.
		if !strings.Contains(page, "Diagnostics") {
			t.Errorf("%s is not in the Diagnostics section", href)
		}
	}
}
