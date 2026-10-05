package exporter_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/exporter"
	"github.com/alternayte/gx/internal/gxstyles"
)

// exportApp is a test app with the exact root page, a page with params and
// static inputs, a signal (so the page loads scripts), a stylesheet and a
// public file.
func exportApp(t *testing.T, extra map[string]string) string {
	t.Helper()
	files := map[string]string{
		"go.mod":                  "module app\n\ngo 1.25.0\n\nrequire github.com/alternayte/gx v0.0.0\n\nreplace github.com/alternayte/gx => " + filepath.ToSlash(repoRoot(t)) + "\n",
		"gx.toml":                 "[site]\nurl = \"https://example.test\"\ntitle = \"Export\"\n",
		"products/route/route.go": "package route\n\nimport \"github.com/alternayte/gx\"\n\ntype Home struct {\n\tgx.Route `GET /{$}`\n}\n\ntype Show struct {\n\tgx.Route `GET /products/{id}`\n\tID int64\n}\n",
		"products/page.go":        "package products\n\nimport (\n\t\"github.com/alternayte/gx\"\n\t\"app/products/route\"\n)\n\nvar HomePage = gx.Page(func(c *gx.Ctx, in route.Home) (HomeViewProps, error) {\n\treturn HomeViewProps{}, nil\n}, HomeView)\n\nvar ShowPage = gx.Page(func(c *gx.Ctx, in route.Show) (ShowViewProps, error) {\n\treturn ShowViewProps{ID: in.ID}, nil\n}, ShowView).Static(func() ([]route.Show, error) {\n\treturn []route.Show{{ID: 1}, {ID: 2}}, nil\n})\n\nvar Routes = gx.Collect(HomePage, ShowPage)\n",
		"products/HomeView.gx":    "package products\n\nimport \"app/products/route\"\n\nsignals {\n  Open bool = false\n}\n\n<main>\n  <h1 id=\"home\">Hello export</h1>\n  <a href={route.Show{ID: 1}}>One</a>\n  <button on:click={$Open = !$Open}>Menu</button>\n  <p show={$Open}>Open</p>\n</main>\n",
		"products/ShowView.gx":    "package products\n\nprops {\n  ID int64\n}\n\n<h1>Product {p.ID}</h1>\n",
		"gxstyles/styles_gx.go":   string(gxstyles.Generate([]byte(".exported-rule{color:red}"))),
		"public/favicon.svg":      "<svg xmlns=\"http://www.w3.org/2000/svg\"><title>favicon</title></svg>\n",
		"assets.go":               "// Package assets embeds the public files of the app.\npackage assets\n\nimport (\n\t\"embed\"\n\t\"io/fs\"\n)\n\n//go:embed public\nvar files embed.FS\n\n// Public returns the embedded public directory.\nfunc Public() fs.FS {\n\tsub, err := fs.Sub(files, \"public\")\n\tif err != nil {\n\t\tpanic(err)\n\t}\n\treturn sub\n}\n",
		"cmd/app/main.go":         "package main\n\nimport (\n\t\"net/http\"\n\t\"os\"\n\n\t\"github.com/alternayte/gx\"\n\t\"github.com/alternayte/gx/adapters/datastar\"\n\t\"app\"\n\t\"app/gxstyles\"\n\t\"app/products\"\n)\n\nfunc main() {\n\tgx.SetStylesheet(gxstyles.CSS())\n\tapp := gx.New(gx.Config{Adapter: datastar.Adapter(), Public: assets.Public()})\n\tapp.Group(\"/\", products.Routes)\n\t_ = http.ListenAndServe(os.Getenv(\"GX_DEV_ADDR\"), app)\n}\n",
	}
	for rel, body := range extra {
		files[rel] = body
	}
	return writeTree(t, files)
}

var hashedAsset = regexp.MustCompile(`/_gx/([a-z]+)\.([0-9a-f]{8})\.(js|css)`)

// TestREQ_EXP_01_PagesInputsAndHashedAssets covers the export of an app:
// every GET page without params, every input from .Static, content-hashed
// assets that the pages name, the public files, 404.html and sitemap.xml
// (REQ-EXP-01).
func TestREQ_EXP_01_PagesInputsAndHashedAssets(t *testing.T) {
	dir := exportApp(t, nil)
	out := filepath.Join(dir, "dist")
	res, err := exporter.Export(context.Background(), exporter.Options{
		Dir: dir,
		Out: out,
		Index: func(ctx context.Context, root, siteDir string) error {
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	read := func(rel string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		return string(data)
	}

	if got := strings.Join(res.Paths, " "); got != "/ /products/1 /products/2" {
		t.Fatalf("exported paths = %q", got)
	}
	home := read("index.html")
	if !strings.Contains(home, "Hello export") {
		t.Fatalf("index.html = %s", home)
	}
	for _, rel := range []string{"products/1/index.html", "products/2/index.html"} {
		if page := read(rel); !strings.Contains(page, "Product ") {
			t.Fatalf("%s = %s", rel, page)
		}
	}

	// Each asset the page names has its content hash in the name.
	named := map[string]bool{}
	for _, m := range hashedAsset.FindAllStringSubmatch(home, -1) {
		named[m[1]+"."+m[3]] = true
		body := read(strings.TrimPrefix(m[0], "/"))
		sum := sha256.Sum256([]byte(body))
		if got := hex.EncodeToString(sum[:])[:8]; got != m[2] {
			t.Fatalf("%s holds content with hash %s", m[0], got)
		}
	}
	for _, want := range []string{"gx.js", "datastar.js", "app.css"} {
		if !named[want] {
			t.Fatalf("index.html names no hashed %s:\n%s", want, home)
		}
	}
	for _, plain := range []string{`"/_gx/gx.js"`, `"/_gx/datastar.js"`, `"/_gx/app.css"`} {
		if strings.Contains(home, plain) {
			t.Fatalf("index.html names the unhashed %s", plain)
		}
	}
	entries, err := os.ReadDir(filepath.Join(out, "_gx"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !hashedAsset.MatchString("/_gx/" + e.Name()) {
			t.Fatalf("_gx/%s has no content hash", e.Name())
		}
	}
	if css := hashedAsset.FindString(home[strings.Index(home, "stylesheet"):]); !strings.Contains(read(strings.TrimPrefix(css, "/")), ".exported-rule") {
		t.Fatalf("the stylesheet %q lacks the app rule", css)
	}

	// Public files keep their names: a host and a browser ask for them.
	if !strings.Contains(read("favicon.svg"), "favicon") {
		t.Fatal("favicon.svg is not the public file")
	}
	notFound := read("404.html")
	if strings.Contains(notFound, "Hello export") {
		t.Fatalf("404.html is the home page:\n%s", notFound)
	}
	if strings.Contains(notFound, `"/_gx/app.css"`) {
		t.Fatalf("404.html names an unhashed asset:\n%s", notFound)
	}
	sitemap := read("sitemap.xml")
	for _, want := range []string{"<loc>https://example.test/</loc>", "<loc>https://example.test/products/2</loc>"} {
		if !strings.Contains(sitemap, want) {
			t.Fatalf("sitemap.xml lacks %s:\n%s", want, sitemap)
		}
	}
}
