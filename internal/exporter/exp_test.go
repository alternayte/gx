package exporter_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/alternayte/gx/internal/exporter"
	"github.com/alternayte/gx/internal/gxstyles"
)

// exportApp is a test app with the exact root page, a page with params and
// static inputs, a signal (so the page loads scripts), a stylesheet and a
// public file.
func exportApp(t *testing.T, extra map[string]string) string {
	t.Helper()
	return writeTree(t, exportAppFiles(t, extra))
}

// exportAppFiles returns the files of the export app.
func exportAppFiles(t *testing.T, extra map[string]string) map[string]string {
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
		"products/Shell.gx":       "package products\n\nimport \"app/products/route\"\n\nprops {\n  Children gx.Node\n}\n\n<nav><a href={route.Home{}}>Home</a></nav>\n<main>{p.Children}</main>\n",
		"products/shell.go":       "package products\n\nimport \"github.com/alternayte/gx\"\n\nvar ShellLayout = gx.Layout(nil, func(_ struct{}, children gx.Node) gx.Node {\n\treturn Shell(ShellProps{Children: children})\n})\n",
		"track/route/route.go":    "package route\n\nimport \"github.com/alternayte/gx\"\n\ntype Page struct {\n\tgx.Route `GET /track`\n}\n\ntype Track struct {\n\tgx.Route `POST /track/{id}`\n\tID int64\n}\n",
		"track/track.go":          "package track\n\nimport (\n\t\"github.com/alternayte/gx\"\n\t\"app/track/route\"\n)\n\nvar Page = gx.Page(func(c *gx.Ctx, in route.Page) (TrackViewProps, error) {\n\treturn TrackViewProps{}, nil\n}, TrackView)\n\nvar Track = gx.Action(func(c *gx.Ctx, in route.Track) error { return nil }).External(\"https://api.example.test/v1/\")\n\nvar Routes = gx.Collect(Page, Track)\n",
		"track/TrackView.gx":      "package track\n\nimport \"app/track/route\"\n\n<button on:click={route.Track{ID: 7}}>Track</button>\n",
		"cmd/app/main.go":         "package main\n\nimport (\n\t\"net/http\"\n\t\"os\"\n\n\t\"github.com/alternayte/gx\"\n\t\"github.com/alternayte/gx/adapters/datastar\"\n\t\"app\"\n\t\"app/gxstyles\"\n\t\"app/products\"\n\t\"app/track\"\n)\n\nfunc main() {\n\tgx.SetStylesheet(gxstyles.CSS())\n\tapp := gx.New(gx.Config{Adapter: datastar.Adapter(), Public: assets.Public()})\n\tapp.Group(\"/\", products.ShellLayout, gx.Nav(gx.MorphNavigation), products.Routes, track.Routes)\n\t_ = http.ListenAndServe(os.Getenv(\"GX_DEV_ADDR\"), app)\n}\n",
	}
	for rel, body := range extra {
		files[rel] = body
	}
	return files
}

var goodExport struct {
	once sync.Once
	out  string
	res  *exporter.Result
	err  error
}

// exportedApp exports the export app once for every test that reads its
// output; one build keeps the package light.
func exportedApp(t *testing.T) (string, *exporter.Result) {
	t.Helper()
	goodExport.once.Do(func() {
		dir, err := os.MkdirTemp("", "gx-export-test-")
		if err != nil {
			goodExport.err = err
			return
		}
		for rel, body := range exportAppFiles(t, nil) {
			path := filepath.Join(dir, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				goodExport.err = err
				return
			}
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				goodExport.err = err
				return
			}
		}
		goodExport.out = filepath.Join(dir, "dist")
		goodExport.res, goodExport.err = exporter.Export(context.Background(), exporter.Options{
			Dir:   dir,
			Out:   goodExport.out,
			Index: func(ctx context.Context, root, siteDir string) error { return nil },
		})
	})
	if goodExport.err != nil {
		t.Fatalf("Export: %v", goodExport.err)
	}
	return goodExport.out, goodExport.res
}

// TestMain removes the shared export.
func TestMain(m *testing.M) {
	code := m.Run()
	if goodExport.out != "" {
		_ = os.RemoveAll(filepath.Dir(goodExport.out))
	}
	os.Exit(code)
}

var hashedAsset = regexp.MustCompile(`/_gx/([a-z]+)\.([0-9a-f]{8})\.(js|css)`)

// TestREQ_EXP_01_PagesInputsAndHashedAssets covers the export of an app:
// every GET page without params, every input from .Static, content-hashed
// assets that the pages name, the public files, 404.html and sitemap.xml
// (REQ-EXP-01).
func TestREQ_EXP_01_PagesInputsAndHashedAssets(t *testing.T) {
	out, res := exportedApp(t)
	read := func(rel string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		return string(data)
	}

	if got := strings.Join(res.Paths, " "); got != "/ /products/1 /products/2 /track" {
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

// serverOnlyApp adds an action, a GET action, an external action, a form
// and a page with live validation to the export app.
func serverOnlyApp(t *testing.T) string {
	t.Helper()
	return exportApp(t, map[string]string{
		"join/route/route.go": "package route\n\nimport \"github.com/alternayte/gx\"\n\ntype Page struct {\n\tgx.Route `GET /join`\n}\n\ntype Join struct {\n\tgx.Route `POST /join`\n\tEmail string\n}\n\nfunc (in *Join) Rules() gx.Rules {\n\treturn gx.Rules{gx.Field(&in.Email, gx.Required, gx.Email)}\n}\n\ntype Add struct {\n\tgx.Route `POST /cart/add`\n}\n\ntype Lazy struct {\n\tgx.Route `GET /lazy`\n}\n\ntype Track struct {\n\tgx.Route `POST /track`\n}\n",
		"join/join.go":        "package join\n\nimport (\n\t\"github.com/alternayte/gx\"\n\t\"app/join/route\"\n)\n\nvar Create = gx.Form(func(c *gx.Ctx, in *route.Join) error { return nil }, JoinView)\n\nvar Page = gx.Page(func(c *gx.Ctx, in route.Page) (JoinViewProps, error) {\n\treturn Create.Props(&route.Join{}), nil\n}, JoinView)\n\nvar Add = gx.Action(func(c *gx.Ctx, in route.Add) error { return nil })\n\nvar Lazy = gx.Action(func(c *gx.Ctx, in route.Lazy) error { return nil })\n\nvar Track = gx.Action(func(c *gx.Ctx, in route.Track) error { return nil }).External(\"https://api.example.test\")\n\nvar Routes = gx.Collect(Create, Page, Add, Lazy, Track)\n",
		"join/JoinView.gx":    "package join\n\nimport \"app/join/route\"\n\nprops {\n  F route.JoinForm\n}\n\n<form {...p.F.Attrs()}>\n  <label for={p.F.Email.ID}>Email</label>\n  <input {...p.F.Email.Attrs()} type=\"email\" data-gx-validate=\"blur\" />\n  <p id={p.F.Email.ID + \"-error\"} role=\"alert\">{p.F.Email.Error}</p>\n  <button type=\"submit\" on:click={route.Track{}}>Join</button>\n</form>\n",
		"cmd/app/main.go":     "package main\n\nimport (\n\t\"net/http\"\n\t\"os\"\n\n\t\"github.com/alternayte/gx\"\n\t\"github.com/alternayte/gx/adapters/datastar\"\n\t\"app/join\"\n\t\"app/products\"\n)\n\nfunc main() {\n\tapp := gx.New(gx.Config{Adapter: datastar.Adapter()})\n\tapp.Group(\"/\", products.Routes, join.Routes)\n\t_ = http.ListenAndServe(os.Getenv(\"GX_DEV_ADDR\"), app)\n}\n",
	})
}

// TestREQ_EXP_02_ServerOnlyReport covers the failure of an export: actions
// without .External, forms and live validation are listed, an external
// action is not, and nothing is written (REQ-EXP-02).
func TestREQ_EXP_02_ServerOnlyReport(t *testing.T) {
	dir := serverOnlyApp(t)
	out := filepath.Join(dir, "dist")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "keep.txt"), []byte("the last good export"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := exporter.Export(context.Background(), exporter.Options{
		Dir: dir,
		Out: out,
		Index: func(ctx context.Context, root, siteDir string) error {
			t.Error("the search index ran on a failed export")
			return nil
		},
	})
	if err == nil {
		t.Fatal("the export of an app with server-only features passed")
	}
	want := `gx export: these features need a server. A static host cannot run them.
  action           GET /lazy
  action           POST /cart/add
  form             POST /join
  form on a page   /join
  live validation  /join
Mark an action that another server answers with .External(url). Remove the other features from the exported pages.`
	if got := err.Error(); got != want {
		t.Fatalf("report:\n%s\nwant:\n%s", got, want)
	}
	if _, err := os.Stat(filepath.Join(out, "keep.txt")); err != nil {
		t.Fatalf("a failed export changed the output directory: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "index.html")); err == nil {
		t.Fatal("a failed export wrote pages")
	}
}

// TestREQ_EXP_02_ExternalAction covers .External: the action exports, and
// the page invokes it at the other server (REQ-EXP-02).
func TestREQ_EXP_02_ExternalAction(t *testing.T) {
	out, _ := exportedApp(t)
	page, err := os.ReadFile(filepath.Join(out, "track", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(page), "https://api.example.test/v1/track/7") {
		t.Fatalf("the page does not invoke the external URL:\n%s", page)
	}
}

// TestREQ_EXP_02_NavigationFallsBackToLinks covers layout-aware navigation
// in an export: the pages hold plain links and no layout slot, so a click
// is a full load (REQ-EXP-02).
func TestREQ_EXP_02_NavigationFallsBackToLinks(t *testing.T) {
	out, _ := exportedApp(t)
	for _, rel := range []string{"index.html", "products/1/index.html"} {
		page, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(page), "data-gx-slot") {
			t.Fatalf("%s holds a layout slot, so the runtime would fetch a patch:\n%s", rel, page)
		}
		if !strings.Contains(string(page), `<a href="/"`) {
			t.Fatalf("%s lost its plain link:\n%s", rel, page)
		}
	}
}
