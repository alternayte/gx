package site_test

import (
	"strings"
	"testing"

	"github.com/alternayte/gx"
	"github.com/alternayte/gx/adapters/datastar"
	"github.com/alternayte/gx/content"
	"github.com/alternayte/gx/docs/site"
)

// TestREQ_DOC_02_Sections covers the docs site as a reader sees it: each
// section is in the sidebar in order, each page answers, the home page
// links to each section, and the tutorial steps link to the next step
// (REQ-DOC-02).
func TestREQ_DOC_02_Sections(t *testing.T) {
	t.Chdir("..") // the collection reads content/ under the module root
	content.Install()
	app := gx.New(gx.Config{Adapter: datastar.Adapter()})
	app.Group("/", site.Routes)

	// The text of the page with each tag as one bar: a sidebar label is
	// alone between two tags.
	page := strings.Join(strings.Fields(tags.ReplaceAllString(get(t, app, "/start/quick-start/"), "|")), " ")
	at := strings.Index(page, "gx-sidebar")
	if at < 0 {
		at = 0
	}
	for _, label := range []string{"Start", "Tutorial", "Guides", "Reference", "Compare", "Diagnostics", "Components"} {
		next := strings.Index(page[at:], "| "+label+" |")
		if next < 0 {
			next = strings.Index(page[at:], "|"+label+"|")
		}
		if next < 0 {
			t.Fatalf("the sidebar lacks the section %q after offset %d", label, at)
		}
		at += next
	}
	for _, path := range []string{
		"/tutorial/a-page/", "/tutorial/ship/",
		"/guides/components/", "/guides/routing/", "/guides/actions-and-signals/", "/guides/forms/",
		"/guides/styling/", "/guides/registry/", "/guides/agent-tools/", "/guides/dev-loop-and-editors/",
		"/guides/content-sites/", "/guides/static-export/", "/guides/security/",
		"/reference/api/", "/reference/cli/", "/reference/grammar/", "/reference/diagnostics/",
		"/compare/comparisons/", "/compare/migrate-from-templ/", "/components/",
	} {
		body := get(t, app, path)
		if !strings.Contains(body, `href="`+path+`"`) {
			t.Errorf("%s is not in its own sidebar", path)
		}
	}
	home := get(t, app, "/")
	for _, href := range []string{"/start/quick-start/", "/tutorial/a-page/", "/guides/components/", "/reference/cli/", "/components/", "/compare/comparisons/"} {
		if !strings.Contains(home, `href="`+href+`"`) {
			t.Errorf("the home page does not link to %s", href)
		}
	}
	// The steps of the tutorial follow each other.
	step := get(t, app, "/tutorial/a-page/")
	if !strings.Contains(step, `href="/tutorial/a-parameter/"`) {
		t.Error("the first tutorial step does not link to the second")
	}
	if !strings.Contains(get(t, app, "/tutorial/an-action/"), "2. A page with a parameter") {
		t.Error("the third tutorial step does not name the step before it")
	}
}
