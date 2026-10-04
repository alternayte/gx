package exporter_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/exporter"
)

// repoRoot returns the gx checkout root for the test module's replace.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

// writeTree writes a test module and returns its root.
func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, body := range files {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// TestREQ_EXP_01_StaticExport covers `gx export`: every GET page renders to
// a file, the assets copy, 404.html writes and the search index runs over
// the output (REQ-EXP-01).
func TestREQ_EXP_01_StaticExport(t *testing.T) {
	mod := "module app\n\ngo 1.25.0\n\nrequire github.com/alternayte/gx v0.0.0\n\nreplace github.com/alternayte/gx => " + filepath.ToSlash(repoRoot(t)) + "\n"
	dir := writeTree(t, map[string]string{
		"go.mod":               mod,
		"products/routes.go":   "package products\n\nimport \"github.com/alternayte/gx\"\n\ntype Home struct {\n\tgx.Route `GET /home`\n}\n\nvar Routes = gx.Collect(HomePage)\n",
		"products/page.go":     "package products\n\nimport \"github.com/alternayte/gx\"\n\nvar HomePage = gx.Page(func(c *gx.Ctx, in Home) (HomeViewProps, error) {\n\treturn HomeViewProps{}, nil\n}, HomeView)\n",
		"products/HomeView.gx": "package products\n\n<h1 id=\"home\">Hello export</h1>\n",
		"main.go":              "package main\n\nimport (\n\t\"net/http\"\n\t\"os\"\n\n\t\"github.com/alternayte/gx\"\n\t\"app/products\"\n)\n\nfunc main() {\n\tapp := gx.New(gx.Config{})\n\tapp.Group(\"/\", products.Routes)\n\t_ = http.ListenAndServe(os.Getenv(\"GX_DEV_ADDR\"), app)\n}\n",
	})
	out := filepath.Join(dir, "dist")
	indexed := ""
	res, err := exporter.Export(context.Background(), exporter.Options{
		Dir:  dir,
		Out:  out,
		Main: ".",
		Index: func(ctx context.Context, root, siteDir string) error {
			indexed = siteDir
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if len(res.Paths) == 0 {
		t.Fatalf("manifest is empty: %+v", res)
	}
	home, err := os.ReadFile(filepath.Join(out, "home", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(home), "Hello export") || !strings.HasPrefix(string(home), "<!doctype html>") {
		t.Fatalf("home/index.html = %q", home)
	}
	if _, err := os.Stat(filepath.Join(out, "404.html")); err != nil {
		t.Fatalf("404.html: %v", err)
	}
	if indexed != out {
		t.Fatalf("index ran on %q, want %q", indexed, out)
	}
}

// TestREQ_CNT_08_LLMS covers the llms.txt export: the three files, the raw
// .md copy of every page, a page skipped through frontmatter and a section
// removed with <docs.LLMSkip> (REQ-CNT-08).
func TestREQ_CNT_08_LLMS(t *testing.T) {
	mod := "module app\n\ngo 1.25.0\n\nrequire github.com/alternayte/gx v0.0.0\n\nreplace github.com/alternayte/gx => " + filepath.ToSlash(repoRoot(t)) + "\n"
	dir := writeTree(t, map[string]string{
		"go.mod":                 mod,
		"content/docs/index.md":  "---\ntitle: Home\ndescription: The docs home.\n---\n\n# Home\n\nStart here.\n",
		"content/docs/start.md":  "---\ntitle: Introduction\ndescription: Start here.\n---\n\n# Introduction\n\nWelcome.\n\n<docs.LLMSkip>\nSecret setup text.\n</docs.LLMSkip>\n\nMore text.\n",
		"content/docs/hidden.md": "---\ntitle: Hidden\ndescription: Not for models.\nllms: skip\n---\n\n# Hidden\n\nHidden body.\n",
		"products/content.go":    "package products\n\nimport (\n\t\"github.com/alternayte/gx\"\n\t\"github.com/alternayte/gx/content\"\n)\n\ntype DocMeta struct {\n\tTitle       string `yaml:\"title\"`\n\tDescription string `yaml:\"description\"`\n\tLLMS        string `yaml:\"llms\"`\n}\n\nvar Docs = gx.Collection[DocMeta](\"content/docs\")\n\nfunc View(e gx.Entry[DocMeta]) gx.Node {\n\treturn gx.El(\"h1\", nil, gx.Text(e.Meta.Title))\n}\n\nvar Routes = gx.Collect(gx.ContentEntries(Docs, View).LLMS(gx.LLMSOptions[DocMeta]{\n\tSite:        \"Deedbox\",\n\tSummary:     \"Event sourcing docs.\",\n\tTitle:       func(m DocMeta) string { return m.Title },\n\tDescription: func(m DocMeta) string { return m.Description },\n\tSkip:        func(m DocMeta) bool { return m.LLMS == \"skip\" },\n}))\n\nfunc init() { content.Install() }\n",
		"main.go":                "package main\n\nimport (\n\t\"net/http\"\n\t\"os\"\n\n\t\"github.com/alternayte/gx\"\n\t\"app/products\"\n)\n\nfunc main() {\n\tapp := gx.New(gx.Config{})\n\tapp.Group(\"/\", products.Routes)\n\t_ = http.ListenAndServe(os.Getenv(\"GX_DEV_ADDR\"), app)\n}\n",
	})
	out := filepath.Join(dir, "dist")
	_, err := exporter.Export(context.Background(), exporter.Options{
		Dir:  dir,
		Out:  out,
		Main: ".",
		Index: func(ctx context.Context, root, siteDir string) error {
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	read := func(name string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(name)))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return string(data)
	}
	llms := read("llms.txt")
	for _, want := range []string{"# Deedbox", "> Event sourcing docs.", "- [Introduction](/start/): Start here."} {
		if !strings.Contains(llms, want) {
			t.Errorf("llms.txt lacks %q:\n%s", want, llms)
		}
	}
	if strings.Contains(llms, "Hidden") {
		t.Errorf("llms.txt holds the skipped page:\n%s", llms)
	}
	full := read("llms-full.txt")
	for _, want := range []string{"Welcome.", "More text."} {
		if !strings.Contains(full, want) {
			t.Errorf("llms-full.txt lacks %q:\n%s", want, full)
		}
	}
	for _, bad := range []string{"Secret setup text.", "Hidden body."} {
		if strings.Contains(full, bad) {
			t.Errorf("llms-full.txt holds %q:\n%s", bad, full)
		}
	}
	small := read("llms-small.txt")
	if !strings.Contains(small, "- [Introduction](/start/)") {
		t.Errorf("llms-small.txt lacks the link:\n%s", small)
	}
	if strings.Contains(small, "Start here.") {
		t.Errorf("llms-small.txt holds descriptions:\n%s", small)
	}
	start := read("start.md")
	if !strings.Contains(start, "Welcome.") || strings.Contains(start, "Secret setup text.") {
		t.Errorf("start.md = %q", start)
	}
	hidden := read("hidden.md")
	if !strings.Contains(hidden, "Hidden body.") {
		t.Errorf("hidden.md = %q", hidden)
	}
}

// TestREQ_CNT_09_HeadMeta covers the exported head, sitemap.xml and
// robots.txt: charset, title template, description, canonical, Open Graph
// and Twitter card (REQ-CNT-09).
func TestREQ_CNT_09_HeadMeta(t *testing.T) {
	mod := "module app\n\ngo 1.25.0\n\nrequire github.com/alternayte/gx v0.0.0\n\nreplace github.com/alternayte/gx => " + filepath.ToSlash(repoRoot(t)) + "\n"
	dir := writeTree(t, map[string]string{
		"go.mod":                mod,
		"gx.toml":               "[site]\nurl = \"https://docs.example.com\"\ntitle = \"Deedbox docs\"\ntitle_template = \"%s | Deedbox docs\"\ndescription = \"Event sourcing docs.\"\n",
		"content/docs/index.md": "---\ntitle: Home\ndescription: The docs home.\n---\n\n# Home\n",
		"content/docs/start.md": "---\ntitle: Introduction\ndescription: Start here.\n---\n\n# Introduction\n",
		"products/content.go":   "package products\n\nimport (\n\t\"github.com/alternayte/gx\"\n\t\"github.com/alternayte/gx/content\"\n)\n\ntype DocMeta struct {\n\tTitle       string `yaml:\"title\"`\n\tDescription string `yaml:\"description\"`\n}\n\nvar Docs = gx.Collection[DocMeta](\"content/docs\")\n\nfunc View(e gx.Entry[DocMeta]) gx.Node {\n\treturn gx.El(\"h1\", nil, gx.Text(e.Meta.Title))\n}\n\nvar Routes = gx.Collect(gx.ContentEntries(Docs, View).LLMS(gx.LLMSOptions[DocMeta]{\n\tSite:        \"Deedbox\",\n\tSummary:     \"Event sourcing docs.\",\n\tTitle:       func(m DocMeta) string { return m.Title },\n\tDescription: func(m DocMeta) string { return m.Description },\n}))\n\nfunc init() { content.Install() }\n",
		"main.go":               "package main\n\nimport (\n\t\"net/http\"\n\t\"os\"\n\n\t\"github.com/alternayte/gx\"\n\t\"app/products\"\n)\n\nfunc main() {\n\tapp := gx.New(gx.Config{})\n\tapp.Group(\"/\", products.Routes)\n\t_ = http.ListenAndServe(os.Getenv(\"GX_DEV_ADDR\"), app)\n}\n",
	})
	out := filepath.Join(dir, "dist")
	_, err := exporter.Export(context.Background(), exporter.Options{
		Dir:  dir,
		Out:  out,
		Main: ".",
		Index: func(ctx context.Context, root, siteDir string) error {
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	page, err := os.ReadFile(filepath.Join(out, "start", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	wantHead := `<meta charset="utf-8"><title>Introduction | Deedbox docs</title><meta name="description" content="Start here."><link rel="canonical" href="https://docs.example.com/start/"><meta property="og:url" content="https://docs.example.com/start/"><meta property="og:title" content="Introduction | Deedbox docs"><meta property="og:description" content="Start here."><meta property="og:type" content="website"><meta name="twitter:card" content="summary"><meta name="twitter:title" content="Introduction | Deedbox docs"><meta name="twitter:description" content="Start here.">`
	if !strings.Contains(string(page), "<head>"+wantHead+"</head>") {
		t.Fatalf("head:\n%s", page)
	}
	if strings.Contains(string(page), "<body><title>") {
		t.Fatalf("inline title stayed in the body:\n%s", page)
	}
	sitemap, err := os.ReadFile(filepath.Join(out, "sitemap.xml"))
	if err != nil {
		t.Fatal(err)
	}
	wantSitemap := "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">\n  <url><loc>https://docs.example.com/</loc></url>\n  <url><loc>https://docs.example.com/start/</loc></url>\n</urlset>\n"
	if string(sitemap) != wantSitemap {
		t.Fatalf("sitemap = %q, want %q", sitemap, wantSitemap)
	}
	robots, err := os.ReadFile(filepath.Join(out, "robots.txt"))
	if err != nil {
		t.Fatal(err)
	}
	wantRobots := "User-agent: *\nAllow: /\n\nSitemap: https://docs.example.com/sitemap.xml\n"
	if string(robots) != wantRobots {
		t.Fatalf("robots = %q, want %q", robots, wantRobots)
	}
}
