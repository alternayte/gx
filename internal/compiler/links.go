package compiler

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	mdparser "github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// mdLink is one Markdown link of a content file.
type mdLink struct {
	dest string
	line int
	col  int
}

// markdownLinks returns every Markdown link of a content file in source
// order (REQ-CNT-10).
func markdownLinks(src []byte) []mdLink {
	doc := goldmark.New().Parser().Parse(text.NewReader(src))
	var out []mdLink
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		link, ok := n.(*ast.Link)
		if !ok {
			return ast.WalkContinue, nil
		}
		at := 0
		if t, ok := link.FirstChild().(*ast.Text); ok && t != nil {
			at = t.Segment.Start
		}
		line, col := offsetPos(src, at)
		out = append(out, mdLink{dest: string(link.Destination), line: line, col: col})
		return ast.WalkContinue, nil
	})
	return out
}

// offsetPos turns a byte offset into a 1-based line and column.
func offsetPos(src []byte, off int) (int, int) {
	if off < 0 || off > len(src) {
		off = 0
	}
	line, col := 1, 1
	for i := 0; i < off; i++ {
		if src[i] == '\n' {
			line++
			col = 1
			continue
		}
		col++
	}
	return line, col
}

// headingAnchors returns the heading ids of a Markdown file (REQ-CNT-10).
func headingAnchors(src []byte) map[string]bool {
	md := goldmark.New(goldmark.WithParserOptions(mdparser.WithAutoHeadingID()))
	doc := md.Parser().Parse(text.NewReader(src))
	out := map[string]bool{}
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		h, ok := n.(*ast.Heading)
		if !ok {
			return ast.WalkContinue, nil
		}
		if v, ok := h.AttributeString("id"); ok {
			if b, ok := v.([]byte); ok {
				out[string(b)] = true
			}
		}
		return ast.WalkContinue, nil
	})
	return out
}

// checkCollectionLinks reports a broken internal page link or heading
// anchor in one collection (GX8003, REQ-CNT-10).
func checkCollectionLinks(coll contentCollection, files map[string][]byte) []Diagnostic {
	slugs := collectionSlugs(coll.dir, files)
	anchors := map[string]map[string]bool{}
	anchorFor := func(path string) map[string]bool {
		if anchors[path] == nil {
			anchors[path] = headingAnchors(files[path])
		}
		return anchors[path]
	}
	var out []Diagnostic
	for path, src := range files {
		slug := slugOf(coll.dir, path)
		dir := slugDir(slug)
		for _, link := range markdownLinks(src) {
			dest := strings.TrimSpace(link.dest)
			if dest == "" || isExternalDest(dest) || strings.HasPrefix(dest, "mailto:") || strings.HasPrefix(dest, "tel:") {
				continue
			}
			pathPart, anchor := splitFragment(dest)
			target := ""
			switch {
			case pathPart == "":
				target = path
			case strings.HasPrefix(pathPart, "/"):
				if hasFileExt(pathPart) && !strings.HasSuffix(pathPart, ".md") {
					continue // an asset, not a page
				}
				// A site path holds the mount prefix of the collection.
				inside, isEntry := pathPart, true
				if coll.prefix != "" {
					inside, isEntry = strings.CutPrefix(pathPart, coll.prefix)
					isEntry = isEntry && (inside == "" || strings.HasPrefix(inside, "/"))
				}
				if isEntry {
					target = slugs[strings.Trim(strings.TrimSuffix(inside, ".md"), "/")]
				}
				if target == "" && coll.appPage != nil && coll.appPage(pathPart) {
					continue // a page route of the app, outside the collection
				}
			default:
				if hasFileExt(pathPart) && !strings.HasSuffix(pathPart, ".md") {
					continue
				}
				if strings.HasSuffix(pathPart, ".md") {
					target = slugs[strings.TrimSuffix(joinSlug(dir, pathPart), ".md")]
				} else {
					target = slugs[joinSlug(dir, pathPart)]
				}
			}
			if target == "" {
				out = append(out, Diagnostic{
					Code: CodeContentLink,
					File: path,
					Line: link.line,
					Col:  link.col,
					Msg:  "link " + Quoted(dest) + " points at no page",
				})
				continue
			}
			if anchor != "" && !anchorFor(target)[anchor] {
				out = append(out, Diagnostic{
					Code: CodeContentLink,
					File: path,
					Line: link.line,
					Col:  link.col,
					Msg:  "link " + Quoted(dest) + " points at no heading",
				})
			}
		}
	}
	return out
}

// checkExternalLinks requests every external link and reports a failure
// (GX8003, REQ-CNT-10). It only runs on request.
func checkExternalLinks(colls []contentCollection, read func(string) ([]byte, error), client *http.Client) []Diagnostic {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	type target struct {
		file string
		line int
		col  int
	}
	urls := map[string][]target{}
	for _, coll := range colls {
		for _, path := range markdownFiles(coll.dir) {
			src, err := readContent(path, read)
			if err != nil {
				continue
			}
			for _, link := range markdownLinks(src) {
				if !isExternalDest(link.dest) {
					continue
				}
				urls[link.dest] = append(urls[link.dest], target{file: path, line: link.line, col: link.col})
			}
		}
	}
	var out []Diagnostic
	for url, at := range urls {
		ok := true
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			ok = false
		} else {
			resp, err := client.Do(req)
			if err != nil {
				ok = false
			} else {
				_ = resp.Body.Close()
				ok = resp.StatusCode < 400
			}
		}
		if ok {
			continue
		}
		for _, t := range at {
			out = append(out, Diagnostic{
				Code: CodeContentLink,
				File: t.file,
				Line: t.line,
				Col:  t.col,
				Msg:  "external link " + Quoted(url) + " does not answer",
			})
		}
	}
	return out
}

// readContent reads one file through the overlay or from disk.
func readContent(path string, read func(string) ([]byte, error)) ([]byte, error) {
	if read != nil {
		return read(path)
	}
	return os.ReadFile(path)
}

// collectionSlugs maps an entry slug to its file. The root index has the
// empty slug, and "guides/index.md" answers "guides" and "guides/index".
func collectionSlugs(dir string, files map[string][]byte) map[string]string {
	out := map[string]string{}
	for path := range files {
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			continue
		}
		slug := strings.TrimSuffix(filepath.ToSlash(rel), ".md")
		out[slug] = path
		switch {
		case slug == "index":
			out[""] = path
		case strings.HasSuffix(slug, "/index"):
			out[strings.TrimSuffix(slug, "/index")] = path
		}
	}
	return out
}

// slugOf returns the entry slug of a file path.
func slugOf(dir, path string) string {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return ""
	}
	return strings.TrimSuffix(filepath.ToSlash(rel), ".md")
}

// slugDir returns the directory part of a slug.
func slugDir(slug string) string {
	if i := strings.LastIndexByte(slug, '/'); i >= 0 {
		return slug[:i]
	}
	return ""
}

// joinSlug joins a slug directory and a relative link path.
func joinSlug(dir, rel string) string {
	return strings.Trim(filepath.ToSlash(filepath.Join(dir, rel)), "/")
}

// splitFragment splits a link into its path and its anchor.
func splitFragment(dest string) (string, string) {
	if i := strings.IndexByte(dest, '#'); i >= 0 {
		return dest[:i], dest[i+1:]
	}
	return dest, ""
}

// hasFileExt reports whether the last path segment has a file extension.
func hasFileExt(path string) bool {
	base := path
	if i := strings.LastIndexByte(base, '/'); i >= 0 {
		base = base[i+1:]
	}
	return strings.Contains(base, ".")
}

// isExternalDest reports whether a link target is an absolute http(s) URL.
func isExternalDest(dest string) bool {
	return strings.HasPrefix(dest, "http://") || strings.HasPrefix(dest, "https://")
}
