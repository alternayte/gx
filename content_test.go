package gx_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/gx"
	"github.com/alternayte/gx/content"
)

type docMeta struct {
	Title string `yaml:"title"`
	Order int    `yaml:"order"`
}

// TestREQ_CNT_02_Collection covers the collection loaders: typed frontmatter,
// list, get, filter and sort (REQ-CNT-02).
func TestREQ_CNT_02_Collection(t *testing.T) {
	content.Install()
	dir := t.TempDir()
	for rel, body := range map[string]string{
		"a.md":   "---\ntitle: Alpha\norder: 2\n---\n\n# Alpha\n",
		"b/c.md": "---\ntitle: Beta\norder: 1\n---\n\n# Beta\n",
	} {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	coll := gx.Collection[docMeta](dir).Components()
	entries := coll.Entries()
	if len(entries) != 2 || entries[0].Slug != "a" || entries[1].Slug != "b/c" {
		t.Fatalf("entries = %+v", entries)
	}
	if entries[0].Meta.Title != "Alpha" || entries[0].Meta.Order != 2 {
		t.Fatalf("meta = %+v", entries[0].Meta)
	}
	if entry, ok := coll.Get("b/c"); !ok || entry.Meta.Title != "Beta" {
		t.Fatalf("Get = %+v, %v", entry, ok)
	}
	if _, ok := coll.Get("missing"); ok {
		t.Fatal("Get found a missing entry")
	}
	filtered := coll.Filter(func(e gx.Entry[docMeta]) bool { return e.Meta.Order > 1 })
	if len(filtered) != 1 || filtered[0].Slug != "a" {
		t.Fatalf("filtered = %+v", filtered)
	}
	sorted := coll.Sorted(func(a, b gx.Entry[docMeta]) bool { return a.Meta.Order < b.Meta.Order })
	if sorted[0].Meta.Order != 1 || sorted[1].Meta.Order != 2 {
		t.Fatalf("sorted = %+v", sorted)
	}
}

// TestREQ_CNT_02_ContentPages covers gx.ContentPages: one route per entry
// with typed frontmatter and a rendered body, and the export input list
// (REQ-CNT-02).
func TestREQ_CNT_02_ContentPages(t *testing.T) {
	content.Install()
	dir := t.TempDir()
	body := "---\ntitle: Alpha\norder: 1\n---\n\n# Alpha\n\nText.\n"
	if err := os.WriteFile(filepath.Join(dir, "a.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	coll := gx.Collection[docMeta](dir).Components()
	h := gx.ContentPages(coll, func(m docMeta, body []byte) gx.Node {
		node, err := content.Body(body)
		if err != nil {
			return gx.Text("body error: " + err.Error())
		}
		return gx.El("main", nil, gx.El("h1", nil, gx.Text(m.Title)), node)
	})
	if h.Pattern() != "/{slug...}" {
		t.Fatalf("pattern = %q", h.Pattern())
	}
	mux := http.NewServeMux()
	mux.Handle("GET /docs/{slug}", h)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/docs/a", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	got := rec.Body.String()
	for _, want := range []string{"<h1>Alpha</h1>", `id="alpha"`, "<p>Text.</p>"} {
		if !strings.Contains(got, want) {
			t.Fatalf("body lacks %q:\n%s", want, got)
		}
	}
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/docs/missing", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing status = %d", rec.Code)
	}
	inputs, ok, err := gx.StaticInputs(h)
	if err != nil || !ok || len(inputs) != 1 {
		t.Fatalf("static inputs = %+v, %v, %v", inputs, ok, err)
	}
	if page, ok := inputs[0].(gx.ContentPage); !ok || page.Slug != "a" {
		t.Fatalf("static input = %#v", inputs[0])
	}
}

// TestREQ_CNT_06_ContentEntryAndHeadings covers the entry view, the index
// route of the docs shell and the heading list for its table of contents
// (REQ-CNT-06).
func TestREQ_CNT_06_ContentEntryAndHeadings(t *testing.T) {
	content.Install()
	dir := t.TempDir()
	files := map[string]string{
		"index.md": "---\ntitle: Home\n---\n\n# Home\n",
		"start.md": "---\ntitle: Start\n---\n\n# Start\n\n## Install\n\n### Database\n\n#### Deep\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	coll := gx.Collection[docMeta](dir).Components()
	h := gx.ContentEntries(coll, func(e gx.Entry[docMeta]) gx.Node {
		return gx.El("h1", nil, gx.Text(e.Meta.Title+":"+e.Slug))
	})
	mux := http.NewServeMux()
	mux.Handle("GET /docs/{slug...}", h)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/docs/", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Home:index") {
		t.Fatalf("index = %d %q", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/docs/start/", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Start:start") {
		t.Fatalf("trailing slash = %d %q", rec.Code, rec.Body.String())
	}
	body := []byte("# Start\n\n## Install\n\n### Database\n\n#### Deep\n")
	headings := content.Headings(body)
	var got []string
	for _, heading := range headings {
		got = append(got, heading.Text+"/"+heading.ID)
	}
	want := []string{"Start/start", "Install/install", "Database/database", "Deep/deep"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("headings = %v, want %v", got, want)
	}
}
