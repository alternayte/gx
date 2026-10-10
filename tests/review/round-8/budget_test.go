package round8_test

import (
	"context"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/exporter"
	"github.com/alternayte/gx/internal/gxconfig"
)

// TestREQ_DEV_13_ImportsOfAModuleOnEachPage checks REQ-DEV-13: "`gx check`
// fails for a route over its budget and names the files."
//
// Two pages load the module /widget.js, which imports ./chunk.js, as a
// module of an island imports the chunk that esbuild wrote. Each page loads
// 137 gzipped bytes of JS in the two files. The budget is 100 bytes. The
// check reports the first page only. For the second page it has the size of
// /widget.js from the first page and does not read its imports again, so
// the page has 68 bytes and no finding: a route over its budget passes
// `gx check`.
func TestREQ_DEV_13_ImportsOfAModuleOnEachPage(t *testing.T) {
	dir := scratchModule(t, map[string]string{
		"gx.toml":             "[site]\nurl = \"https://example.test\"\ntitle = \"Budget\"\n",
		"site/route/route.go": "package route\n\nimport \"github.com/alternayte/gx\"\n\ntype Home struct {\n\tgx.Route `GET /{$}`\n}\n\ntype About struct {\n\tgx.Route `GET /about`\n}\n",
		"site/page.go": "package site\n\nimport (\n\t\"github.com/alternayte/gx\"\n\t\"app/site/route\"\n)\n\n" +
			"var HomePage = gx.Page(func(c *gx.Ctx, in route.Home) (ViewProps, error) { return ViewProps{Title: \"Home\"}, nil }, View)\n" +
			"var AboutPage = gx.Page(func(c *gx.Ctx, in route.About) (ViewProps, error) { return ViewProps{Title: \"About\"}, nil }, View)\n\n" +
			"var Routes = gx.Collect(HomePage, AboutPage)\n",
		"site/View.gx":     "package site\n\nimport \"app/site/route\"\n\nprops {\n  Title string\n}\n\n<main>\n  <h1>{p.Title}</h1>\n  <a href={route.Home{}}>Home</a>\n  <a href={route.About{}}>About</a>\n  <script type=\"module\" src=\"/widget.js\"></script>\n</main>\n",
		"public/widget.js": "import \"./chunk.js\";\nconsole.log(\"widget\");\n",
		"public/chunk.js":  "console.log(\"" + strings.Repeat("chunk of the widget ", 40) + "\");\n",
		"assets.go":        "// Package assets embeds the public files of the app.\npackage assets\n\nimport (\n\t\"embed\"\n\t\"io/fs\"\n)\n\n//go:embed public\nvar files embed.FS\n\n// Public returns the embedded public directory.\nfunc Public() fs.FS {\n\tsub, err := fs.Sub(files, \"public\")\n\tif err != nil {\n\t\tpanic(err)\n\t}\n\treturn sub\n}\n",
		"cmd/app/main.go":  "package main\n\nimport (\n\t\"net/http\"\n\t\"os\"\n\n\t\"github.com/alternayte/gx\"\n\t\"github.com/alternayte/gx/adapters/datastar\"\n\t\"app\"\n\t\"app/site\"\n)\n\nfunc main() {\n\tapp := gx.New(gx.Config{Adapter: datastar.Adapter(), Public: assets.Public()})\n\tapp.Group(\"/\", site.Routes)\n\t_ = http.ListenAndServe(os.Getenv(\"GX_DEV_ADDR\"), app)\n}\n",
	})
	findings, err := exporter.Budgets(context.Background(), dir, "", gxconfig.Budget{JS: 100})
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]exporter.BudgetFinding{}
	for _, f := range findings {
		by[f.Path] = f
	}
	home, ok := by["/"]
	if !ok || len(home.Files) != 2 || home.Bytes <= 100 {
		t.Fatalf("the fixture is wrong: the home page loads two files over the budget, and the findings are %v", findings)
	}
	about, ok := by["/about"]
	if !ok {
		t.Fatalf("the page /about loads the same two files as the page / (%d bytes, budget 100), and the check has no finding for it: %v", home.Bytes, findings)
	}
	if about.Bytes != home.Bytes || len(about.Files) != 2 {
		t.Errorf("the page /about loads the same files as the page /: %+v, want %d bytes in two files", about, home.Bytes)
	}
}
