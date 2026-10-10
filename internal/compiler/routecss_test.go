package compiler_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TestREQ_STY_13_RouteClasses checks the class list of each page route: the
// classes of the packages that the page can reach, with the layout and the
// toast of the app, and no class of a slice that the page does not import. A
// page that can use each class of the app has no list of its own
// (REQ-STY-13).
func TestREQ_STY_13_RouteClasses(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod": moduleWithGx(t),
		// The layout of each page, with a component of its own package.
		"shell/Shell.gx": "package shell\n\nprops {\n  Children gx.Node\n}\n\n<main class=\"shell-main\"><Bar />{p.Children}</main>\n",
		"shell/Bar.gx":   "package shell\n\n<nav class=\"shell-bar\">Site</nav>\n",
		"shell/shell.go": "package shell\n\nimport \"github.com/alternayte/gx\"\n\nvar Layout = gx.Layout(func(c *gx.Ctx) (struct{}, error) { return struct{}{}, nil },\n\tfunc(_ struct{}, children gx.Node) gx.Node { return Shell(ShellProps{Children: children}) })\n",
		"shell/toast.go": "package shell\n\nimport \"github.com/alternayte/gx\"\n\nfunc Toast(p gx.ToastPatch) gx.Node { return gx.El(\"p\", gx.Attrs{{Key: \"class\", Value: toastClass}}) }\n\nconst toastClass = \"toast-box\"\n",
		// A shared component package.
		"ui/badge/Badge.gx": "package badge\n\n<span class=\"badge-pill\">New</span>\n",
		// The cart slice: its page uses the badge, and its action patches
		// a component with a class in a Go file.
		"cart/route/r.go": "package route\n\nimport \"github.com/alternayte/gx\"\n\ntype Page struct {\n\tgx.Route `GET /cart`\n}\n\ntype Add struct {\n\tgx.Route `POST /cart/add`\n}\n",
		"cart/View.gx":    "package cart\n\nimport \"app/ui/badge\"\n\n<section class=\"cart-view\"><badge.Badge /><button on:click={route.Add{}}>Add</button></section>\n",
		"cart/Line.gx":    "package cart\n\nprops {\n  Class string\n}\n\n<p id=\"cart-line\" class={p.Class}>Line</p>\n",
		"cart/cart.go":    "package cart\n\nimport (\n\t\"app/cart/route\"\n\n\t\"github.com/alternayte/gx\"\n)\n\nvar Page = gx.Page(func(c *gx.Ctx, in route.Page) (ViewProps, error) { return ViewProps{}, nil }, View)\n\nvar Add = gx.Action(func(c *gx.Ctx, in route.Add) error { return c.Patch(Line(LineProps{Class: \"cart-line-new\"})) })\n\nvar Routes = gx.Collect(Page, Add)\n",
		// The blog slice does not import the cart or the badge.
		"blog/route/r.go": "package route\n\nimport \"github.com/alternayte/gx\"\n\ntype Page struct {\n\tgx.Route `GET /blog`\n}\n",
		"blog/View.gx":    "package blog\n\n<article class=\"blog-view\">Post</article>\n",
		"blog/blog.go":    "package blog\n\nimport (\n\t\"app/blog/route\"\n\n\t\"github.com/alternayte/gx\"\n)\n\nvar Page = gx.Page(func(c *gx.Ctx, in route.Page) (ViewProps, error) { return ViewProps{}, nil }, View)\n\nvar Routes = gx.Collect(Page)\n",
		// The home page is in the package that imports each slice.
		"site/route/r.go": "package route\n\nimport \"github.com/alternayte/gx\"\n\ntype Home struct {\n\tgx.Route `GET /`\n}\n",
		"site/Home.gx":    "package site\n\n<h1 class=\"home-title\">Home</h1>\n",
		"site/site.go":    "package site\n\nimport (\n\t\"app/blog\"\n\t\"app/cart\"\n\t\"app/shell\"\n\t\"app/site/route\"\n\n\t\"github.com/alternayte/gx\"\n)\n\nvar Home = gx.Page(func(c *gx.Ctx, in route.Home) (HomeProps, error) { return HomeProps{}, nil }, HomeView)\n\nvar HomeView = func(p HomeProps) gx.Node { return nil }\n\nfunc Mount(app *gx.App) {\n\tapp.Group(\"/\", shell.Layout, gx.Collect(Home), cart.Routes, blog.Routes)\n}\n\nvar Config = gx.Config{Toast: shell.Toast}\n",
	})
	// The cart view names route.Add: it needs the import.
	view := filepath.Join(dir, "cart/View.gx")
	src, _ := os.ReadFile(view)
	if err := os.WriteFile(view, []byte("package cart\n\nimport (\n  \"app/cart/route\"\n  \"app/ui/badge\"\n)\n"+string(src)[len("package cart\n\nimport \"app/ui/badge\"\n"):]), 0o644); err != nil {
		t.Fatal(err)
	}
	files := generateFiles(t, dir)
	data := files[filepath.Join(dir, ".gx/route-classes.json")]
	var lists map[string][]string
	if err := json.Unmarshal(data, &lists); err != nil {
		t.Fatalf("route-classes.json: %v: %s", err, data)
	}
	// A list holds each string literal of its packages, as the class list
	// of the app does (REQ-STY-02): Tailwind reads the classes from them.
	common := []string{"shell-bar", "shell-main", "toast-box"}
	for pattern, c := range map[string]struct{ has, not []string }{
		"GET /cart": {append([]string{"badge-pill", "cart-line-new", "cart-view"}, common...), []string{"blog-view", "home-title"}},
		"GET /blog": {append([]string{"blog-view"}, common...), []string{"badge-pill", "cart-line-new", "cart-view", "home-title"}},
	} {
		got := lists[pattern]
		for _, class := range c.has {
			if !slices.Contains(got, class) {
				t.Errorf("%s: the list lacks %s: %v", pattern, class, got)
			}
		}
		for _, class := range c.not {
			if slices.Contains(got, class) {
				t.Errorf("%s: the list holds %s, a class of a package that the page does not reach", pattern, class)
			}
		}
		if !slices.IsSorted(got) {
			t.Errorf("%s: the list is not in order", pattern)
		}
	}
	// The view of the home page is not a generated component, so the
	// compiler does not see what it renders: no list.
	if _, ok := lists["GET /"]; ok || len(lists) != 2 {
		t.Errorf("lists = %v, want the cart and the blog only", lists)
	}
}
