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

// TestREQ_RTE_14_NestedRouteLists covers a route list built from other
// lists: every page and action in an appended list reports the mount of
// the Group call that holds the outer list (REQ-RTE-14).
func TestREQ_RTE_14_NestedRouteLists(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":               moduleWithGx(t),
		"products/routes.go":   routesGo,
		"products/page.go":     routesPageGo,
		"products/ShowView.gx": "package products\n\nprops {\n  ID   int64\n  Tab  string\n  Page int64\n}\n\n<p>x</p>\n",
		"cart/route/route.go":  "package route\n\nimport \"github.com/alternayte/gx\"\n\ntype Add struct {\n\tgx.Route `POST /cart/add`\n}\n",
		"cart/cart.go":         "package cart\n\nimport (\n\t\"github.com/alternayte/gx\"\n\t\"app/cart/route\"\n)\n\nvar Add = gx.Action(func(c *gx.Ctx, in route.Add) error { return nil })\n\nvar Routes = gx.Collect(Add)\n",
		"main.go": "package main\n\nimport (\n\t\"github.com/alternayte/gx\"\n\t\"app/cart\"\n\t\"app/products\"\n)\n\n" +
			"var shopLayout = gx.Layout(func(c *gx.Ctx) (int, error) { return 0, nil }, func(_ int, children gx.Node) gx.Node { return children })\n\n" +
			"var all = append(append(gx.Collect(), products.Routes...), cart.Routes...)\n\n" +
			"func main() {\n\tapp := gx.New(gx.Config{})\n\tapp.Group(\"/shop\", shopLayout, all)\n}\n",
	})
	reports, diags := compiler.Routes(dir)
	if len(diags) > 0 {
		t.Fatalf("Routes diagnostics: %v", diags)
	}
	if len(reports) != 2 {
		t.Fatalf("reports = %+v, want two routes", reports)
	}
	for _, r := range reports {
		if r.Prefix != "/shop" || strings.Join(r.Layouts, ",") != "shopLayout" {
			t.Fatalf("%s has no mount: %+v", r.Type, r)
		}
	}
}
