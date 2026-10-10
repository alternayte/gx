package exporter

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/alternayte/gx/internal/apprun"
	"github.com/alternayte/gx/internal/gxconfig"
)

// The budget check of gx check (REQ-DEV-13). It runs the app as gx export
// does, reads each page, and adds the gzipped size of the JS files and of
// the stylesheets that the page loads. The numbers are what a browser gets
// for the first view of the page.

// BudgetFinding is one page over one of its budgets.
type BudgetFinding struct {
	// Pattern is the route pattern of the page, and Path the address that
	// the check read.
	Pattern, Path string
	// Kind is "JS" or "CSS".
	Kind string
	// Bytes is the gzipped size that the page loads, and Limit its budget.
	Bytes, Limit int
	// Files are the files of the page of that kind, the largest first.
	Files []BudgetFile
}

// BudgetFile is one file of a page with its gzipped size.
type BudgetFile struct {
	URL   string
	Bytes int
}

// String is the message of the finding: the numbers and each file.
func (f BudgetFinding) String() string {
	parts := make([]string, len(f.Files))
	for i, file := range f.Files {
		parts[i] = fmt.Sprintf("%s %d", file.URL, file.Bytes)
	}
	return fmt.Sprintf("the page %s of the route %s loads %d bytes of %s (gzipped), and its budget is %d: %s",
		f.Path, f.Pattern, f.Bytes, f.Kind, f.Limit, strings.Join(parts, ", "))
}

var (
	scriptSrc = regexp.MustCompile(`<script\b[^>]*\bsrc="([^"]+)"`)
	sheetHref = regexp.MustCompile(`<link\b[^>]*\brel="stylesheet"[^>]*\bhref="([^"]+)"|<link\b[^>]*\bhref="([^"]+)"[^>]*\brel="stylesheet"`)
	// An island and an imported web component name their module in an
	// attribute; the loader imports it.
	moduleSrc = regexp.MustCompile(`<gx-island\b[^>]*\bsrc="([^"]+)"|\bdata-gx-module="([^"]+)"`)
	// A static import of a module that esbuild wrote.
	importFrom = regexp.MustCompile(`(?:from|import)\s*"(\.{1,2}/[^"]+)"`)
)

// Budgets starts the app of dir and returns each page that is over its
// budget. main is the main package of the app, or "" to find it.
func Budgets(ctx context.Context, dir, main string, budget gxconfig.Budget) ([]BudgetFinding, error) {
	if !budget.Set() {
		return nil, nil
	}
	app, err := apprun.Start(ctx, dir, main)
	if err != nil {
		return nil, fmt.Errorf("gx check: the budget check cannot run the app: %w", err)
	}
	defer app.Stop()
	manifest, err := fetchManifest(ctx, app.Base)
	if err != nil {
		return nil, err
	}
	// The pattern of a page: the routes of the budget table are in a mux,
	// which says which one an address has.
	mux := http.NewServeMux()
	for pattern := range budget.Routes {
		func() {
			// A pattern that the mux refuses has no page.
			defer func() { _ = recover() }()
			mux.Handle(pattern, http.NotFoundHandler())
		}()
	}
	sizes := map[string]int{}
	size := func(ref string) (int, []string, error) {
		if n, ok := sizes[ref]; ok {
			return n, nil, nil
		}
		resp, err := httpGet(ctx, app.Base+ref)
		if err != nil {
			return 0, nil, err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return 0, nil, err
		}
		if resp.StatusCode != http.StatusOK {
			return 0, nil, fmt.Errorf("gx check: %s: %s", ref, resp.Status)
		}
		var buf bytes.Buffer
		zw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
		_, _ = zw.Write(body)
		_ = zw.Close()
		sizes[ref] = buf.Len()
		var imports []string
		if strings.HasSuffix(strings.SplitN(ref, "?", 2)[0], ".js") {
			base, _ := url.Parse(ref)
			for _, m := range importFrom.FindAllSubmatch(body, -1) {
				if next, err := base.Parse(string(m[1])); err == nil {
					imports = append(imports, next.String())
				}
			}
		}
		return buf.Len(), imports, nil
	}
	var out []BudgetFinding
	paths := append([]string(nil), manifest.Paths...)
	sort.Strings(paths)
	for _, path := range paths {
		req, err := http.NewRequest(http.MethodGet, path, nil)
		if err != nil {
			continue
		}
		_, pattern := mux.Handler(req)
		limits := gxconfig.RouteBudget{JS: budget.JS, CSS: budget.CSS}
		if route, ok := budget.Routes[pattern]; ok && pattern != "" {
			if route.JS >= 0 {
				limits.JS = route.JS
			}
			if route.CSS >= 0 {
				limits.CSS = route.CSS
			}
		}
		if pattern == "" {
			pattern = "GET " + path
		}
		if limits.JS <= 0 && limits.CSS <= 0 {
			continue
		}
		body, status, err := fetchPage(ctx, app.Base, path)
		if err != nil {
			return nil, err
		}
		if status != http.StatusOK {
			continue
		}
		collect := func(kind string, limit int, refs []string) error {
			if limit <= 0 {
				return nil
			}
			seen := map[string]bool{}
			var files []BudgetFile
			total := 0
			for len(refs) > 0 {
				ref := refs[0]
				refs = refs[1:]
				if seen[ref] || !strings.HasPrefix(ref, "/") || strings.HasPrefix(ref, "//") {
					// A file of a different origin is not a file of the app.
					continue
				}
				seen[ref] = true
				n, imports, err := size(ref)
				if err != nil {
					return err
				}
				total += n
				files = append(files, BudgetFile{URL: ref, Bytes: n})
				refs = append(refs, imports...)
			}
			if total > limit {
				sort.SliceStable(files, func(i, j int) bool { return files[i].Bytes > files[j].Bytes })
				out = append(out, BudgetFinding{Pattern: pattern, Path: path, Kind: kind, Bytes: total, Limit: limit, Files: files})
			}
			return nil
		}
		var scripts, sheets []string
		for _, m := range scriptSrc.FindAllSubmatch(body, -1) {
			scripts = append(scripts, string(m[1]))
		}
		for _, m := range moduleSrc.FindAllSubmatch(body, -1) {
			scripts = append(scripts, string(m[1])+string(m[2]))
		}
		for _, m := range sheetHref.FindAllSubmatch(body, -1) {
			sheets = append(sheets, string(m[1])+string(m[2]))
		}
		if err := collect("JS", limits.JS, scripts); err != nil {
			return nil, err
		}
		if err := collect("CSS", limits.CSS, sheets); err != nil {
			return nil, err
		}
	}
	return out, nil
}
