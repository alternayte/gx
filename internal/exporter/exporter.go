// Package exporter renders a Gx app to static files (REQ-EXP-01). It builds
// the app with the gxdev tag, asks the dev-only export manifest for every
// GET page, writes each page under the output directory, copies the /_gx/
// assets, writes 404.html and builds the Pagefind index.
package exporter

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/alternayte/gx"
	"github.com/alternayte/gx/internal/apprun"
	"github.com/alternayte/gx/internal/gxconfig"
	"github.com/alternayte/gx/internal/pagefind"
)

// Options configure one export.
type Options struct {
	// Dir is the app module root.
	Dir string
	// Out is the output directory. It is emptied first.
	Out string
	// Main is the main package path; DetectMain finds it when empty.
	Main string
	// Log receives progress lines. Defaults to io.Discard.
	Log io.Writer
	// Index writes the search index into a directory. Defaults to the
	// pinned Pagefind binary.
	Index func(ctx context.Context, root, siteDir string) error
	// SiteURL overrides [site] url from gx.toml (REQ-CNT-09).
	SiteURL string
}

// Manifest is the dev-only export listing (REQ-EXP-01, REQ-CNT-08).
type Manifest struct {
	Paths  []string         `json:"paths"`
	Assets []string         `json:"assets"`
	LLMS   *gx.LLMSManifest `json:"llms"`
	// ServerOnly lists the mounted routes that need a server
	// (REQ-EXP-02).
	ServerOnly []ServerFeature `json:"serverOnly"`
}

// ServerFeature is one feature a static host cannot run (REQ-EXP-02).
type ServerFeature struct {
	// Kind is "action", "form", "form on a page" or "live validation".
	Kind string `json:"kind"`
	// Pattern is the route pattern, or the page path for a page marker.
	Pattern string `json:"pattern"`
}

// Result carries the exported pages for the follow-up writers
// (REQ-CNT-08, REQ-CNT-09).
type Result struct {
	// Pages maps a site path to its rendered HTML.
	Pages map[string][]byte
	// Paths lists the exported site paths in manifest order.
	Paths []string
	// Assets lists the /_gx/ asset URLs.
	Assets []string
	// NotFound is the rendered 404 page.
	NotFound []byte
	// LLMS is the llms.txt manifest of the app, when it has one.
	LLMS *gx.LLMSManifest
}

// Export builds and renders the app into opt.Out (REQ-EXP-01).
func Export(ctx context.Context, opt Options) (*Result, error) {
	if opt.Log == nil {
		opt.Log = io.Discard
	}
	dir, err := filepath.Abs(opt.Dir)
	if err != nil {
		return nil, err
	}
	if opt.Out == "" {
		opt.Out = filepath.Join(dir, "dist")
	}
	out, err := filepath.Abs(opt.Out)
	if err != nil {
		return nil, err
	}
	app, err := apprun.Start(ctx, dir, opt.Main)
	if err != nil {
		return nil, fmt.Errorf("gx export: %w", err)
	}
	defer app.Stop()
	base := app.Base

	manifest, err := fetchManifest(ctx, base)
	if err != nil {
		return nil, err
	}
	// Every page renders before anything is written: a page can hold a
	// feature that needs a server, and a failed export must leave the
	// last good output alone (REQ-EXP-02).
	res := &Result{Pages: map[string][]byte{}, Paths: manifest.Paths, Assets: manifest.Assets}
	features := append([]ServerFeature{}, manifest.ServerOnly...)
	for _, path := range manifest.Paths {
		body, status, err := fetchPage(ctx, base, path)
		if err != nil {
			return nil, err
		}
		if status != http.StatusOK {
			return nil, fmt.Errorf("gx export: %s: %s", path, http.StatusText(status))
		}
		res.Pages[path] = body
		features = append(features, pageFeatures(path, body)...)
	}
	if len(features) > 0 {
		return nil, serverOnlyError(features)
	}
	if err := os.RemoveAll(out); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return nil, err
	}
	site, err := gxconfig.Load(dir)
	if err != nil {
		return nil, err
	}
	if opt.SiteURL != "" {
		site.Site.URL = opt.SiteURL
	}
	metas := map[string]gx.LLMSEntry{}
	if manifest.LLMS != nil {
		for _, e := range manifest.LLMS.Entries {
			metas[e.Path] = e
		}
	}
	// The assets go first: a page names each Gx asset by its content hash.
	hashed, err := writeAssets(ctx, base, out, manifest.Assets)
	if err != nil {
		return nil, err
	}
	iw := &imageWriter{dir: dir, out: out}
	for _, path := range manifest.Paths {
		if err := writePage(out, path, hashed.Replace(iw.rewriteImages(res.Pages[path])), site.Site, metas[path]); err != nil {
			return nil, err
		}
		fmt.Fprintf(opt.Log, "exported %s\n", path)
	}
	if err := writeSiteFiles(out, site.Site, manifest.Paths); err != nil {
		return nil, err
	}
	notFound, _, err := fetchPage(ctx, base, "/_gx-export-missing-page")
	if err != nil {
		return nil, err
	}
	notFound = hashed.Replace(notFound)
	res.NotFound = notFound
	if err := os.WriteFile(filepath.Join(out, "404.html"), notFound, 0o644); err != nil {
		return nil, err
	}
	if err := writeLLMS(out, manifest.LLMS); err != nil {
		return nil, err
	}
	res.LLMS = manifest.LLMS
	index := opt.Index
	if index == nil {
		index = func(ctx context.Context, root, siteDir string) error {
			return (&pagefind.Manager{Root: root}).Index(ctx, siteDir)
		}
	}
	if err := index(ctx, dir, out); err != nil {
		return nil, err
	}
	return res, nil
}

// formMarker and validateMarker find a form and a live validation control
// in a rendered page.
var (
	formMarker     = regexp.MustCompile(`<form\b[^>]*\bdata-gx-form\b`)
	validateMarker = regexp.MustCompile(`\bdata-gx-validate="(blur|input)"`)
)

// pageFeatures lists what a rendered page holds that needs a server
// (REQ-EXP-02).
func pageFeatures(path string, body []byte) []ServerFeature {
	var out []ServerFeature
	if formMarker.Match(body) {
		out = append(out, ServerFeature{Kind: "form on a page", Pattern: path})
	}
	if validateMarker.Match(body) {
		out = append(out, ServerFeature{Kind: "live validation", Pattern: path})
	}
	return out
}

// serverOnlyError is the failure report of an export (REQ-EXP-02).
func serverOnlyError(features []ServerFeature) error {
	sort.SliceStable(features, func(i, j int) bool {
		if features[i].Kind != features[j].Kind {
			return features[i].Kind < features[j].Kind
		}
		return features[i].Pattern < features[j].Pattern
	})
	var b strings.Builder
	b.WriteString("gx export: these features need a server. A static host cannot run them.\n")
	for _, f := range features {
		fmt.Fprintf(&b, "  %-15s  %s\n", f.Kind, f.Pattern)
	}
	b.WriteString("Mark an action that another server answers with .External(url). Remove the other features from the exported pages.")
	return errors.New(b.String())
}

// assetNames maps the URL of each Gx asset to its content-hashed URL.
type assetNames map[string]string

// Replace puts the hashed name in every attribute that names a Gx asset.
// An attribute value ends with a quote, so "/_gx/gx.js" never matches
// inside a longer name. A document inside a srcdoc attribute has its quotes
// escaped.
func (names assetNames) Replace(page []byte) []byte {
	for plain, hashed := range names {
		for _, quote := range []string{`"`, "&#34;", "&quot;"} {
			page = bytes.ReplaceAll(page, []byte(plain+quote), []byte(hashed+quote))
		}
	}
	return page
}

// writeAssets copies the assets of the manifest into out (REQ-EXP-01). A
// Gx asset under /_gx/ gets its content hash in its name, so a host can
// cache it without end. An app's public file keeps its name: a browser or
// a crawler asks for favicon.ico and robots.txt by name.
func writeAssets(ctx context.Context, base, out string, assets []string) (assetNames, error) {
	names := assetNames{}
	for _, asset := range assets {
		body, status, err := fetchPage(ctx, base, asset)
		if err != nil {
			return nil, err
		}
		if status != http.StatusOK {
			continue
		}
		target := asset
		if i := strings.LastIndex(asset, "/_gx/"); i >= 0 {
			sum := sha256.Sum256(body)
			ext := filepath.Ext(asset)
			target = strings.TrimSuffix(asset, ext) + "." + hex.EncodeToString(sum[:])[:8] + ext
			names[asset[i:]] = target[i:]
		}
		full := filepath.Join(out, filepath.FromSlash(strings.TrimPrefix(target, "/")))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(full, body, 0o644); err != nil {
			return nil, err
		}
	}
	return names, nil
}

// fetchManifest reads the dev-only export listing.
func fetchManifest(ctx context.Context, base string) (*Manifest, error) {
	resp, err := httpGet(ctx, base+"/_gx/export")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gx export: /_gx/export: %s", resp.Status)
	}
	var m Manifest
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return nil, fmt.Errorf("gx export: /_gx/export: %w", err)
	}
	return &m, nil
}

// fetchPage reads one page from the running app.
func fetchPage(ctx context.Context, base, path string) ([]byte, int, error) {
	resp, err := httpGet(ctx, base+path)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}
	return body, resp.StatusCode, nil
}

// httpGet runs one GET with a timeout.
func httpGet(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	// The app renders a page for a static host: no layout slot, so a link
	// is a full load (REQ-EXP-02).
	req.Header.Set("Gx-Export", "1")
	return (&http.Client{Timeout: 60 * time.Second}).Do(req)
}

// writePage writes one page under the output directory.
func writePage(root, path string, body []byte, site gxconfig.Site, meta gx.LLMSEntry) error {
	rel := strings.TrimPrefix(path, "/")
	full := ""
	switch {
	case rel == "":
		full = filepath.Join(root, "index.html")
	case strings.Contains(filepath.Base(rel), ".") && !strings.HasSuffix(rel, "/"):
		full = filepath.Join(root, filepath.FromSlash(rel))
	default:
		full = filepath.Join(root, filepath.FromSlash(strings.TrimSuffix(rel, "/")), "index.html")
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	title := meta.Title
	if title == "" {
		title = extractTitle(body)
	}
	description := meta.Description
	if description == "" {
		description = site.Description
	}
	return os.WriteFile(full, document(body, buildHead(site, path, site.PageTitle(title), description)), 0o644)
}

// titleTag finds the inline title gx.Head renders.
var titleTag = regexp.MustCompile(`(?is)<title>(.*?)</title>`)

// extractTitle returns the text of the first inline title element.
func extractTitle(body []byte) string {
	m := titleTag.FindSubmatch(body)
	if m == nil {
		return ""
	}
	return html.UnescapeString(strings.TrimSpace(string(m[1])))
}

// buildHead returns the head of one exported page (REQ-CNT-09): charset,
// title, description, canonical, Open Graph and Twitter card.
func buildHead(site gxconfig.Site, path, title, description string) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<title>%s</title>`, html.EscapeString(title))
	if description != "" {
		fmt.Fprintf(&b, `<meta name="description" content="%s">`, html.EscapeString(description))
	}
	if site.URL != "" {
		canonical := strings.TrimSuffix(site.URL, "/") + path
		fmt.Fprintf(&b, `<link rel="canonical" href="%s">`, html.EscapeString(canonical))
		fmt.Fprintf(&b, `<meta property="og:url" content="%s">`, html.EscapeString(canonical))
	}
	fmt.Fprintf(&b, `<meta property="og:title" content="%s">`, html.EscapeString(title))
	if description != "" {
		fmt.Fprintf(&b, `<meta property="og:description" content="%s">`, html.EscapeString(description))
	}
	b.WriteString(`<meta property="og:type" content="website">`)
	b.WriteString(`<meta name="twitter:card" content="summary">`)
	fmt.Fprintf(&b, `<meta name="twitter:title" content="%s">`, html.EscapeString(title))
	if description != "" {
		fmt.Fprintf(&b, `<meta name="twitter:description" content="%s">`, html.EscapeString(description))
	}
	return b.String()
}

// document puts the exported head into the document the app served
// (REQ-EXP-01, REQ-CNT-09). The app writes the document shell; the export
// replaces its title and description with the site forms and adds the
// canonical and social tags. A body with no head is a hand-written
// response and gets a shell of its own.
func document(body []byte, head string) []byte {
	i := bytes.Index(body, []byte("<head>"))
	if i < 0 {
		body = titleTag.ReplaceAll(body, nil)
		doc := make([]byte, 0, len(body)+len(head)+80)
		doc = append(doc, "<!doctype html><html lang=\"en\"><head><meta charset=\"utf-8\">"...)
		doc = append(doc, head...)
		doc = append(doc, "</head><body>"...)
		doc = append(doc, body...)
		doc = append(doc, "</body></html>"...)
		return doc
	}
	i += len("<head>")
	end := bytes.Index(body, []byte("</head>"))
	if end < i {
		end = i
	}
	served := body[i:end]
	served = titleTag.ReplaceAll(served, nil)
	served = descriptionTag.ReplaceAll(served, nil)
	// The charset and the viewport stay first; the exported tags follow.
	lead := charsetTag
	served = bytes.Replace(served, []byte(charsetTag), nil, 1)
	if bytes.HasPrefix(served, []byte(viewportTag)) {
		lead += viewportTag
		served = served[len(viewportTag):]
	}
	doc := make([]byte, 0, len(body)+len(head))
	doc = append(doc, body[:i]...)
	doc = append(doc, lead...)
	doc = append(doc, head...)
	doc = append(doc, served...)
	doc = append(doc, body[end:]...)
	return doc
}

// charsetTag is the charset the document shell writes first.
const charsetTag = `<meta charset="utf-8">`

// viewportTag is the viewport the document shell writes second.
const viewportTag = `<meta name="viewport" content="width=device-width, initial-scale=1">`

// descriptionTag finds the description a page set through gx.Head; the
// export writes its own.
var descriptionTag = regexp.MustCompile(`(?is)<meta name="description"[^>]*>`)

// writeSiteFiles writes robots.txt and sitemap.xml when the site has a URL
// (REQ-CNT-09).
func writeSiteFiles(out string, site gxconfig.Site, paths []string) error {
	if site.URL == "" {
		return nil
	}
	base := strings.TrimSuffix(site.URL, "/")
	var sitemap strings.Builder
	sitemap.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	sitemap.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	for _, path := range paths {
		fmt.Fprintf(&sitemap, "  <url><loc>%s</loc></url>\n", html.EscapeString(base+path))
	}
	sitemap.WriteString("</urlset>\n")
	if err := os.WriteFile(filepath.Join(out, "sitemap.xml"), []byte(sitemap.String()), 0o644); err != nil {
		return err
	}
	robots := "User-agent: *\nAllow: /\n\nSitemap: " + base + "/sitemap.xml\n"
	return os.WriteFile(filepath.Join(out, "robots.txt"), []byte(robots), 0o644)
}

// writeLLMS writes llms.txt, llms-full.txt, llms-small.txt and a raw .md
// copy of every page (REQ-CNT-08).
func writeLLMS(out string, m *gx.LLMSManifest) error {
	if m == nil || len(m.Entries) == 0 {
		return nil
	}
	var index strings.Builder
	fmt.Fprintf(&index, "# %s\n\n", m.Site)
	if m.Summary != "" {
		fmt.Fprintf(&index, "> %s\n\n", m.Summary)
	}
	for _, e := range m.Entries {
		if e.Skip {
			continue
		}
		fmt.Fprintf(&index, "- [%s](%s)", e.Title, e.Path)
		if e.Description != "" {
			fmt.Fprintf(&index, ": %s", e.Description)
		}
		index.WriteString("\n")
	}
	var full strings.Builder
	fmt.Fprintf(&full, "# %s\n\n", m.Site)
	if m.Summary != "" {
		fmt.Fprintf(&full, "> %s\n\n", m.Summary)
	}
	var small strings.Builder
	fmt.Fprintf(&small, "# %s\n\n", m.Site)
	for _, e := range m.Entries {
		if e.Skip {
			continue
		}
		fmt.Fprintf(&full, "---\n\n# %s\n\n%s\n\n", e.Title, strings.TrimSpace(e.Body))
		fmt.Fprintf(&small, "- [%s](%s)\n", e.Title, e.Path)
	}
	files := map[string]string{
		"llms.txt":       index.String(),
		"llms-full.txt":  full.String(),
		"llms-small.txt": small.String(),
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(out, name), []byte(body), 0o644); err != nil {
			return err
		}
	}
	// A raw Markdown copy of every page, skipped pages included.
	for _, e := range m.Entries {
		rel := strings.Trim(e.Path, "/")
		if rel == "" {
			rel = "index"
		}
		full := filepath.Join(out, filepath.FromSlash(rel)+".md")
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, []byte(strings.TrimSpace(e.Body)+"\n"), 0o644); err != nil {
			return err
		}
	}
	return nil
}
