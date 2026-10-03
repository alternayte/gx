package compiler_test

import (
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

func TestREQ_RTE_14_Routes(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":               moduleWithGx(t),
		"products/routes.go":   routesGo,
		"products/page.go":     routesPageGo,
		"products/ShowView.gx": "package products\n\nprops {\n  ID   int64\n  Tab  string\n  Page int64\n}\n\n<p>x</p>\n",
		"main.go": "package main\n\nimport (\n\t\"net/http\"\n\n\t\"github.com/alternayte/gx\"\n\t\"app/products\"\n)\n\n" +
			"var shopLayout = gx.Layout(func(c *gx.Ctx) (int, error) { return 0, nil }, func(_ int, children gx.Node) gx.Node { return children })\n\n" +
			"var logging = func(next http.Handler) http.Handler { return next }\n\n" +
			"func main() {\n\tapp := gx.New(gx.Config{})\n\tapp.Group(\"/shop\", shopLayout, logging, products.Routes)\n}\n",
	})
	reports, diags := compiler.Routes(dir)
	if len(diags) > 0 {
		t.Fatalf("Routes diagnostics: %v", diags)
	}
	if len(reports) != 1 {
		t.Fatalf("reports = %+v, want one route", reports)
	}
	r := reports[0]
	if r.Type != "products.Show" || r.Method != "GET" || r.Pattern != "GET /products/{id}" {
		t.Fatalf("route = %+v", r)
	}
	if r.Page != "products.ShowPage" {
		t.Fatalf("page = %q", r.Page)
	}
	if got := strings.Join(r.Fields, ","); got != "ID int64,Tab string,Page int64" {
		t.Fatalf("fields = %q", got)
	}
	if r.Prefix != "/shop" || strings.Join(r.Layouts, ",") != "shopLayout" || strings.Join(r.Middleware, ",") != "logging" {
		t.Fatalf("mount = %+v", r)
	}
}
