package round7_test

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

// TestREQ_STY_13_ActionOfADifferentSlice checks REQ-STY-13: "For each page
// route the build writes one stylesheet with only the classes of the
// components that the view of the page, its layouts and its actions can
// reach." Acceptance: "A patch of an action brings no element with a class
// that has no rule."
//
// The app has the layout of DR-01: "Route types live in a per-slice `route`
// subpackage ... (cart links to products, products invokes cart)". The
// products page has a button that invokes the action Add of the cart slice,
// so the products slice imports only app/cart/route. The action is in
// package cart and patches the component cart.Added, which has the class
// cart-added-note.
//
// The class list of a page route holds the classes of the package of the
// page, of the package of its view and of the packages that they import.
// Package cart is not one of them. The stylesheet of GET /products then has
// no rule for cart-added-note, and the patch of the action brings an
// element with that class.
func TestREQ_STY_13_ActionOfADifferentSlice(t *testing.T) {
	dir := scratchModule(t, map[string]string{
		"cart/route/r.go": "package route\n\nimport \"github.com/alternayte/gx\"\n\ntype Add struct {\n\tgx.Route `POST /cart/add`\n}\n",
		"cart/Added.gx":   "package cart\n\n<p id=\"notice\" class=\"cart-added-note\">Added</p>\n",
		"cart/cart.go":    "package cart\n\nimport (\n\t\"app/cart/route\"\n\n\t\"github.com/alternayte/gx\"\n)\n\nvar Add = gx.Action(func(c *gx.Ctx, in route.Add) error { return c.Patch(Added(AddedProps{})) })\n\nvar Routes = gx.Collect(Add)\n",

		"products/route/r.go":  "package route\n\nimport \"github.com/alternayte/gx\"\n\ntype Page struct {\n\tgx.Route `GET /products`\n}\n",
		"products/View.gx":     "package products\n\nimport cartroute \"app/cart/route\"\n\n<section class=\"products-view\"><p id=\"notice\"></p><button on:click={cartroute.Add{}}>Add</button></section>\n",
		"products/products.go": "package products\n\nimport (\n\t\"app/products/route\"\n\n\t\"github.com/alternayte/gx\"\n)\n\nvar Page = gx.Page(func(c *gx.Ctx, in route.Page) (ViewProps, error) { return ViewProps{}, nil }, View)\n\nvar Routes = gx.Collect(Page)\n",

		// A third slice, so the class list of the app is larger than the
		// list of each page.
		"blog/route/r.go": "package route\n\nimport \"github.com/alternayte/gx\"\n\ntype Page struct {\n\tgx.Route `GET /blog`\n}\n",
		"blog/View.gx":    "package blog\n\n<article class=\"blog-view\">Post</article>\n",
		"blog/blog.go":    "package blog\n\nimport (\n\t\"app/blog/route\"\n\n\t\"github.com/alternayte/gx\"\n)\n\nvar Page = gx.Page(func(c *gx.Ctx, in route.Page) (ViewProps, error) { return ViewProps{}, nil }, View)\n\nvar Routes = gx.Collect(Page)\n",

		"main.go": "package main\n\nimport (\n\t\"app/blog\"\n\t\"app/cart\"\n\t\"app/products\"\n\n\t\"github.com/alternayte/gx\"\n)\n\nfunc main() {\n\tapp := gx.New(gx.Config{})\n\tapp.Group(\"/\", products.Routes, cart.Routes, blog.Routes)\n}\n",
	})
	files, diags := compiler.Generate(dir)
	if compiler.Failed(diags) {
		t.Fatalf("the fixture does not compile: %v", diags)
	}
	var lists map[string][]string
	if data := files[filepath.Join(dir, ".gx/route-classes.json")]; len(data) > 0 {
		if err := json.Unmarshal(data, &lists); err != nil {
			t.Fatalf("route-classes.json: %v: %s", err, data)
		}
	}
	// A page with no list links the stylesheet of the app, which has each
	// class. A page with a list links a stylesheet with only that list.
	got, own := lists["GET /products"]
	if !own {
		return
	}
	if !slices.Contains(got, "products-view") {
		t.Fatalf("the fixture is wrong: the list of GET /products is %v", got)
	}
	if !slices.Contains(got, "cart-added-note") {
		t.Errorf("the stylesheet of GET /products has the classes %v: it lacks cart-added-note, the class of the component that the action of its button patches into the page", got)
	}
}
