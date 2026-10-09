package site

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestREQ_DOC_04_ResultPages checks the result frames of the guides: each
// Result tag has a capture, and each capture has a tag. A capture with no
// tag is a page that no guide shows.
func TestREQ_DOC_04_ResultPages(t *testing.T) {
	tag := regexp.MustCompile(`<Result page="([^"]+)" get="([^"]+)" />`)
	used := map[string]bool{}
	err := filepath.WalkDir(filepath.Join("..", "content"), func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".md") {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		slug := strings.TrimSuffix(filepath.ToSlash(strings.TrimPrefix(path, filepath.Join("..", "content")+string(filepath.Separator))), ".md")
		for _, m := range tag.FindAllStringSubmatch(string(data), -1) {
			if m[1] != slug {
				t.Errorf("%s: the Result tag names the page %q", slug, m[1])
			}
			key := resultKey(m[1], m[2])
			r, ok := findResult(key)
			if !ok {
				t.Errorf("%s: no capture for GET %s (results/%s.html); run just docs-results", slug, m[2], key)
				continue
			}
			if r.Get != m[2] || strings.TrimSpace(r.Body) == "" || r.CSS == "" {
				t.Errorf("%s: the capture %s is for GET %q, with %d bytes of body and %d bytes of styles", slug, key, r.Get, len(r.Body), len(r.CSS))
			}
			if strings.Contains(r.Body, "<script") || strings.Contains(r.Body, " data-gx-") {
				t.Errorf("%s: the capture %s holds a script or a live attribute", slug, key)
			}
			used[key] = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(used) == 0 {
		t.Fatal("no guide shows a result page")
	}
	for _, key := range resultKeys() {
		if !used[key] {
			t.Errorf("results/%s.html has no Result tag in a guide; run just docs-results", key)
		}
	}
}
