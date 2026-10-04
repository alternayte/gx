package devserver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestREQ_CNT_07_SearchCrawlPaths covers the crawled page layout and the
// index asset content types (REQ-CNT-07).
func TestREQ_CNT_07_SearchCrawlPaths(t *testing.T) {
	root := t.TempDir()
	cases := []struct {
		path string
		file string
	}{
		{"/", "index.html"},
		{"/start/", "start/index.html"},
		{"/guides/routing/", "guides/routing/index.html"},
		{"/errors/GX1000.html", "errors/GX1000.html"},
	}
	for _, c := range cases {
		if err := writeSearchPage(root, c.path, []byte("<h1>Hello</h1>")); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(c.file)))
		if err != nil {
			t.Fatalf("%s -> %s: %v", c.path, c.file, err)
		}
		got := string(data)
		if !strings.HasPrefix(got, "<!doctype html>") || !strings.Contains(got, "<h1>Hello</h1>") {
			t.Fatalf("%s body = %q", c.path, got)
		}
	}

	types := map[string]string{
		"pagefind.js":   "text/javascript; charset=utf-8",
		"pagefind.css":  "text/css; charset=utf-8",
		"entry.json":    "application/json",
		"pagefind.wasm": "application/wasm",
		"fragment.pf":   "application/octet-stream",
	}
	for rel, want := range types {
		if got := searchContentType(rel); got != want {
			t.Errorf("searchContentType(%s) = %q, want %q", rel, got, want)
		}
	}
}
