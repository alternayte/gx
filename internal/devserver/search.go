package devserver

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/alternayte/gx/internal/pagefind"
)

// searchSiteDir is the crawled site the search index is built from.
func (s *server) searchSiteDir() string { return filepath.Join(s.workDir(), "search-site") }

// searchHTMLDir is the output root Pagefind serves from.
func (s *server) searchHTMLDir() string { return filepath.Join(s.searchSiteDir(), "pagefind") }

// searchLink finds one anchor href in a rendered page.
var searchLink = regexp.MustCompile(`(?i)<a\b[^>]*\bhref\s*=\s*["']([^"']+)["']`)

// serveSearch builds the search index on demand and serves the Pagefind
// assets at /pagefind/ (REQ-CNT-07).
func (s *server) serveSearch(w http.ResponseWriter, r *http.Request) {
	if err := s.ensureSearch(r.Context()); err != nil {
		http.Error(w, "gx dev: search: "+err.Error(), http.StatusInternalServerError)
		return
	}
	rel := strings.TrimPrefix(r.URL.Path, "/pagefind/")
	if rel == "" || strings.Contains(rel, "..") {
		http.NotFound(w, r)
		return
	}
	full := filepath.Join(s.searchHTMLDir(), filepath.FromSlash(rel))
	if !strings.HasPrefix(full, s.searchHTMLDir()) {
		http.NotFound(w, r)
		return
	}
	data, err := os.ReadFile(full)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", searchContentType(rel))
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(data)
}

// ensureSearch rebuilds the index when the source tree changed
// (REQ-CNT-07).
func (s *server) ensureSearch(ctx context.Context) error {
	s.searchMu.Lock()
	defer s.searchMu.Unlock()
	snap := s.snapshot()
	if s.searchSnap != nil && sameSnapshot(snap, s.searchSnap) {
		return nil
	}
	if err := s.buildSearch(ctx); err != nil {
		return err
	}
	s.searchSnap = snap
	return nil
}

// buildSearch crawls the running app and indexes every page with Pagefind
// (REQ-CNT-07).
func (s *server) buildSearch(ctx context.Context) error {
	base := "http://" + net.JoinHostPort("127.0.0.1", strconv.Itoa(s.appPort))
	if err := os.RemoveAll(s.searchSiteDir()); err != nil {
		return err
	}
	if err := os.MkdirAll(s.searchHTMLDir(), 0o755); err != nil {
		return err
	}
	client := &http.Client{Timeout: 20 * time.Second}
	seen := map[string]bool{}
	queue := []string{"/"}
	for len(queue) > 0 && len(seen) < 300 {
		path := queue[0]
		queue = queue[1:]
		if seen[path] {
			continue
		}
		seen[path] = true
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+path, nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("crawl %s: %w", path, err)
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK || !strings.Contains(resp.Header.Get("Content-Type"), "text/html") {
			continue
		}
		for _, m := range searchLink.FindAllSubmatch(body, -1) {
			href := string(m[1])
			if !strings.HasPrefix(href, "/") || strings.HasPrefix(href, "//") {
				continue
			}
			if i := strings.IndexAny(href, "?#"); i >= 0 {
				href = href[:i]
			}
			if href == "" || seen[href] {
				continue
			}
			queue = append(queue, href)
		}
		if err := writeSearchPage(s.searchSiteDir(), path, body); err != nil {
			return err
		}
	}
	m := &pagefind.Manager{Root: s.dir}
	return m.Index(ctx, s.searchSiteDir())
}

// writeSearchPage saves one crawled page as the file Pagefind expects for
// its URL.
func writeSearchPage(root, path string, body []byte) error {
	rel := strings.TrimPrefix(path, "/")
	rel = strings.TrimSuffix(rel, "/")
	full := ""
	switch {
	case rel == "":
		full = filepath.Join(root, "index.html")
	case strings.Contains(filepath.Base(rel), "."):
		full = filepath.Join(root, filepath.FromSlash(rel))
	default:
		full = filepath.Join(root, filepath.FromSlash(rel), "index.html")
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	// Gx pages are HTML fragments; Pagefind indexes complete documents.
	doc := append([]byte("<!doctype html><html lang=\"en\"><head><meta charset=\"utf-8\"></head><body>"), body...)
	doc = append(doc, []byte("</body></html>")...)
	return os.WriteFile(full, doc, 0o644)
}

// searchContentType returns the content type of one index asset.
func searchContentType(rel string) string {
	switch {
	case strings.HasSuffix(rel, ".js"):
		return "text/javascript; charset=utf-8"
	case strings.HasSuffix(rel, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(rel, ".json"):
		return "application/json"
	case strings.HasSuffix(rel, ".wasm"):
		return "application/wasm"
	default:
		return "application/octet-stream"
	}
}
