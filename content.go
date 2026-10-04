package gx

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// frontmatterDecoder decodes YAML frontmatter into Meta. The gx/content
// package installs a decoder (REQ-CNT-02); package gx keeps no YAML
// dependency (SDD §3.4).
var frontmatterDecoder func([]byte, any) error

// SetFrontmatterDecoder installs the frontmatter decoder of the process
// (REQ-CNT-02). The gx/content package installs one.
func SetFrontmatterDecoder(fn func([]byte, any) error) { frontmatterDecoder = fn }

// collection is a typed set of Markdown content files (REQ-CNT-02). The
// compiler reads the declaration for content checks; the runtime loads the
// entries.
type collection[Meta any] struct {
	dir        string
	components []any

	mu      sync.Mutex
	entries []Entry[Meta]
	loaded  bool
	err     error
}

// Entry is one loaded content page (REQ-CNT-02).
type Entry[Meta any] struct {
	// Slug is the path below the collection directory without ".md".
	Slug string
	// File is the absolute path of the file.
	File string
	// Meta is the decoded frontmatter.
	Meta Meta
	// Body is the Markdown without the frontmatter.
	Body []byte
}

// Collection declares a content collection rooted at dir, relative to the
// module root.
func Collection[Meta any](dir string) *collection[Meta] {
	return &collection[Meta]{dir: dir}
}

// Components names the components that content files of this collection may
// use (REQ-CNT-03). A component not in this list is a GX8002.
func (c *collection[Meta]) Components(components ...any) *collection[Meta] {
	c.components = append(c.components, components...)
	return c
}

// Dir returns the collection directory as declared.
func (c *collection[Meta]) Dir() string { return c.dir }

// Load reads every .md file below the directory, decodes its frontmatter
// into Meta and checks it with Rules() when Meta has them (REQ-CNT-02).
func (c *collection[Meta]) Load() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.loaded {
		return c.err
	}
	c.loaded = true
	c.entries, c.err = loadEntries[Meta](c.dir)
	return c.err
}

// Entries returns every entry, ordered by slug (REQ-CNT-02).
func (c *collection[Meta]) Entries() []Entry[Meta] {
	_ = c.Load()
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]Entry[Meta](nil), c.entries...)
}

// Get returns one entry by slug, with or without the ".md" suffix and
// with or without a trailing slash (REQ-CNT-02).
func (c *collection[Meta]) Get(slug string) (Entry[Meta], bool) {
	slug = strings.TrimSuffix(slug, ".md")
	slug = strings.TrimSuffix(slug, "/")
	for _, e := range c.Entries() {
		if e.Slug == slug {
			return e, true
		}
	}
	return Entry[Meta]{}, false
}

// Filter returns the entries that keep returns true for (REQ-CNT-02).
func (c *collection[Meta]) Filter(keep func(Entry[Meta]) bool) []Entry[Meta] {
	var out []Entry[Meta]
	for _, e := range c.Entries() {
		if keep(e) {
			out = append(out, e)
		}
	}
	return out
}

// Sorted returns the entries ordered by less (REQ-CNT-02).
func (c *collection[Meta]) Sorted(less func(a, b Entry[Meta]) bool) []Entry[Meta] {
	out := c.Entries()
	sort.SliceStable(out, func(i, j int) bool { return less(out[i], out[j]) })
	return out
}

// loadEntries walks dir and decodes every .md file.
func loadEntries[Meta any](dir string) ([]Entry[Meta], error) {
	var out []Entry[Meta]
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		slug := strings.TrimSuffix(filepath.ToSlash(rel), ".md")
		front, body := splitFrontmatter(data)
		var meta Meta
		if len(front) > 0 {
			if frontmatterDecoder == nil {
				return &ContentError{File: path, Msg: "no frontmatter decoder; import github.com/alternayte/gx/content"}
			}
			if err := frontmatterDecoder(front, &meta); err != nil {
				return &ContentError{File: path, Msg: err.Error()}
			}
		}
		if violations := RunAllRulesContext(context.Background(), &meta); len(violations) > 0 {
			v := violations[0]
			return &ContentError{File: path, Msg: v.Message}
		}
		out = append(out, Entry[Meta]{Slug: slug, File: path, Meta: meta, Body: body})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out, nil
}

// splitFrontmatter returns the YAML frontmatter and the body.
func splitFrontmatter(data []byte) (front, body []byte) {
	s := string(data)
	if !strings.HasPrefix(s, "---\n") && !strings.HasPrefix(s, "---\r\n") {
		return nil, data
	}
	rest := s[strings.IndexByte(s, '\n')+1:]
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return nil, data
	}
	front = []byte(rest[:idx])
	after := rest[idx+1:]
	if nl := strings.IndexByte(after, '\n'); nl >= 0 {
		body = []byte(after[nl+1:])
	}
	return front, body
}

// ContentError is a frontmatter or rule error of one content file
// (REQ-CNT-02).
type ContentError struct {
	File string
	Msg  string
}

func (e *ContentError) Error() string { return e.File + ": " + e.Msg }

// ContentPage is the export input of one content page (REQ-CNT-02).
type ContentPage struct {
	Slug string `path:"slug"`
}

// URL implements the typed link value of a content page (REQ-EXP-01). The
// index entry is the site root.
func (p ContentPage) URL() string {
	if p.Slug == "" || p.Slug == "index" {
		return "/"
	}
	return "/" + p.Slug + "/"
}

// ContentRoute serves one content collection (REQ-CNT-02) and carries the
// configuration of the llms.txt export (REQ-CNT-08).
type ContentRoute[Meta any] struct {
	coll *collection[Meta]
	view func(Entry[Meta]) Node
	llms *LLMSOptions[Meta]
}

// ContentPages makes the content route of the collection (REQ-CNT-02): one
// URL per entry under "/{slug...}". view builds the page from the typed
// frontmatter and the raw Markdown body; the docs kit renders the body.
func ContentPages[Meta any](c *collection[Meta], view func(Meta, []byte) Node) *ContentRoute[Meta] {
	return ContentEntries(c, func(e Entry[Meta]) Node { return view(e.Meta, e.Body) })
}

// ContentEntries makes one route per entry and passes the whole Entry to
// view (REQ-CNT-06). The docs shell needs the slug to build links, the
// table of contents and the previous and next pages.
func ContentEntries[Meta any](c *collection[Meta], view func(Entry[Meta]) Node) *ContentRoute[Meta] {
	return &ContentRoute[Meta]{coll: c, view: view}
}

// LLMS configures the llms.txt files of this collection (REQ-CNT-08).
func (h *ContentRoute[Meta]) LLMS(opt LLMSOptions[Meta]) *ContentRoute[Meta] {
	h.llms = &opt
	return h
}

// Pattern implements Handler (REQ-CNT-02). The trailing wildcard serves a
// nested entry slug ("guides/routing") as one path.
func (h *ContentRoute[Meta]) Pattern() string { return "/{slug...}" }

// ServeHTTP renders the entry named by the path value. The empty slug is
// the collection index (REQ-CNT-06).
func (h *ContentRoute[Meta]) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if slug == "" {
		slug = "index"
	}
	entry, ok := h.coll.Get(slug)
	if !ok {
		renderError(w, r, NotFound())
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := RenderRequest(w, r, h.view(entry)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// hasStatic implements the export source (REQ-CNT-02, REQ-EXP-01).
func (h *ContentRoute[Meta]) hasStatic() bool { return true }

// staticInputs returns every entry slug for the static export.
func (h *ContentRoute[Meta]) staticInputs() ([]any, error) {
	entries := h.coll.Entries()
	out := make([]any, 0, len(entries))
	for _, e := range entries {
		out = append(out, ContentPage{Slug: e.Slug})
	}
	return out, nil
}

// llmsManifest implements the export manifest source (REQ-CNT-08).
func (h *ContentRoute[Meta]) llmsManifest() LLMSManifest {
	if h.llms == nil {
		return LLMSManifest{}
	}
	m := LLMSManifest{Site: h.llms.Site, Summary: h.llms.Summary}
	for _, e := range h.coll.Entries() {
		entry := LLMSEntry{Path: string(ContentPage{Slug: e.Slug}.URL()), Body: stripLLMSkip(string(e.Body))}
		if h.llms.Title != nil {
			entry.Title = h.llms.Title(e.Meta)
		}
		if h.llms.Description != nil {
			entry.Description = h.llms.Description(e.Meta)
		}
		if h.llms.Skip != nil {
			entry.Skip = h.llms.Skip(e.Meta)
		}
		m.Entries = append(m.Entries, entry)
	}
	return m
}
