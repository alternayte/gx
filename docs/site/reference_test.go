package site_test

import (
	"html"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/gx"
	"github.com/alternayte/gx/adapters/datastar"
	"github.com/alternayte/gx/content"
	"github.com/alternayte/gx/docs/registry"
	"github.com/alternayte/gx/docs/site"
)

// propDescriptions reads the props block of one .gx file: each prop name
// with the text of the comment lines above it.
func propDescriptions(t *testing.T, file string) map[string]string {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	var doc []string
	inProps := false
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case !inProps:
			inProps = strings.HasPrefix(trimmed, "props {")
		case trimmed == "}":
			return out
		case strings.HasPrefix(trimmed, "//"):
			doc = append(doc, strings.TrimSpace(strings.TrimPrefix(trimmed, "//")))
		case trimmed != "":
			out[strings.Fields(trimmed)[0]] = strings.Join(doc, " ")
			doc = nil
		}
	}
	return out
}

// TestREQ_DOC_02_APIReference renders the page of every item: the page has
// an API reference, and it holds the description that the props block gives
// each prop of each component (REQ-DOC-02).
func TestREQ_DOC_02_APIReference(t *testing.T) {
	t.Chdir("..")
	content.Install()
	app := gx.New(gx.Config{Adapter: datastar.Adapter()})
	app.Group("/", site.Routes)

	rows := 0
	for name := range registryFixtures(t, filepath.Join("..", "registry")) {
		it, _ := registry.Find(name)
		if it.IconSet {
			continue
		}
		page := html.UnescapeString(tags.ReplaceAllString(get(t, app, "/components/"+name+"/"), ""))
		if !strings.Contains(page, "API reference") {
			t.Errorf("/components/%s/ has no API reference", name)
			continue
		}
		files, _ := filepath.Glob(filepath.Join("..", "registry", name, "*.gx"))
		for _, file := range files {
			// A local .gx state directory of the compiler matches the glob.
			if info, err := os.Stat(file); err == nil && info.IsDir() {
				continue
			}
			for prop, doc := range propDescriptions(t, file) {
				rows++
				if doc == "" {
					t.Errorf("%s: the prop %s has no description", file, prop)
					continue
				}
				if !strings.Contains(page, doc) {
					t.Errorf("/components/%s/ does not show the description of %s in %s: %q", name, prop, filepath.Base(file), doc)
				}
			}
		}
	}
	if rows == 0 {
		t.Fatal("the registry has no props")
	}
}

// TestREQ_DOC_02_ToastPreview renders the preview of a toast example: the
// toast waits in a template, next to the button that copies it and the
// toaster that takes it, and the page loads the behaviour runtime. The page
// of the item shows the action code (REQ-DOC-02).
func TestREQ_DOC_02_ToastPreview(t *testing.T) {
	t.Chdir("..")
	content.Install()
	app := gx.New(gx.Config{Adapter: datastar.Adapter()})
	app.Group("/", site.Routes)

	preview := get(t, app, "/preview/toast/toast-success/")
	template := strings.Index(preview, "<template data-docs-toast-source>")
	if template < 0 {
		t.Fatalf("the toast preview has no template:\n%s", preview)
	}
	end := strings.Index(preview, "</template>")
	if toast := strings.Index(preview, `data-gx-toast=""`); toast < template || toast > end {
		t.Errorf("the toast is not inside the template")
	}
	if toaster := strings.Index(preview, "data-gx-toaster"); toaster < end {
		t.Errorf("the toaster is not after the template: a toast in the toaster shows at once")
	}
	for _, want := range []string{`data-docs-toast=""`, "/_gx/behavior.js", "content.cloneNode(true)"} {
		if !strings.Contains(preview, want) {
			t.Errorf("the toast preview lacks %s", want)
		}
	}

	page := html.UnescapeString(tags.ReplaceAllString(get(t, app, "/components/toast/"), ""))
	for _, want := range []string{`return c.Toast("Changes saved", gx.ToastSuccess)`, "Gx has no client call that shows a toast."} {
		if !strings.Contains(page, want) {
			t.Errorf("/components/toast/ lacks %q", want)
		}
	}
}
