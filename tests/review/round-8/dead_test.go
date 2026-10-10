package round8_test

import (
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

// TestREQ_DEV_14_RouteWithALinkInAnExpression checks REQ-DEV-14: "`gx check
// --dead` prints the components with no caller, the page routes with no
// typed link to them, and the classes of the theme that no file uses."
//
// The home page links to the page About through a prop of a component, in
// the form that registry/email/USAGE.md and docs/content/components/email.md
// give: href={gx.URL(route.About{}.URL())}. It links to the page Third
// through the list of a menu. The page Lost has no link. The report prints
// About and Third as page routes with no typed link: it reads only the type
// of the whole value of an attribute of a .gx file, and each of these
// values has the type gx.URL or []gx.URL. The same expression in a Go file
// counts as a link.
func TestREQ_DEV_14_RouteWithALinkInAnExpression(t *testing.T) {
	dir := scratchModule(t, map[string]string{
		"site/route/route.go": "package route\n\nimport \"github.com/alternayte/gx\"\n\ntype Home struct {\n\tgx.Route `GET /{$}`\n}\n\ntype About struct {\n\tgx.Route `GET /about`\n}\n\ntype Third struct {\n\tgx.Route `GET /third`\n}\n\ntype Lost struct {\n\tgx.Route `GET /lost`\n}\n",
		"site/page.go": "package site\n\nimport (\n\t\"github.com/alternayte/gx\"\n\t\"app/site/route\"\n)\n\n" +
			"var HomePage = gx.Page(func(c *gx.Ctx, in route.Home) (HomeViewProps, error) { return HomeViewProps{}, nil }, HomeView)\n" +
			"var AboutPage = gx.Page(func(c *gx.Ctx, in route.About) (OtherProps, error) { return OtherProps{}, nil }, Other)\n" +
			"var ThirdPage = gx.Page(func(c *gx.Ctx, in route.Third) (OtherProps, error) { return OtherProps{}, nil }, Other)\n" +
			"var LostPage = gx.Page(func(c *gx.Ctx, in route.Lost) (OtherProps, error) { return OtherProps{}, nil }, Other)\n\n" +
			"var Routes = gx.Collect(HomePage, AboutPage, ThirdPage, LostPage)\n",
		"site/HomeView.gx": "package site\n\nimport \"app/site/route\"\n\n<main>\n  <a href={route.Home{}}>Home</a>\n  <Btn href={gx.URL(route.About{}.URL())}>About</Btn>\n  <Menu items={[]gx.URL{gx.URL(route.Third{}.URL())}} />\n</main>\n",
		"site/Other.gx":    "package site\n\n<p>other</p>\n",
		"site/Btn.gx":      "package site\n\nprops {\n  Href     gx.URL\n  Children gx.Node\n}\n\n<a href={p.Href}>{p.Children}</a>\n",
		"site/Menu.gx":     "package site\n\nprops {\n  Items []gx.URL\n}\n\n<nav>\n  for _, it := range p.Items {\n    <a href={it}>Page</a>\n  }\n</nav>\n",
	})
	generateModule(t, dir)
	report, diags := compiler.Dead(dir)
	if report == nil {
		t.Fatalf("no report: %v", diags)
	}
	dead := map[string]bool{}
	for _, r := range report.Routes {
		dead[r.Pattern] = true
	}
	if !dead["GET /lost"] || dead["GET /{$}"] {
		t.Fatalf("the fixture is wrong: the report has the routes %+v", report.Routes)
	}
	for pattern, link := range map[string]string{
		"GET /about": "<Btn href={gx.URL(route.About{}.URL())}>",
		"GET /third": "<Menu items={[]gx.URL{gx.URL(route.Third{}.URL())}} />",
	} {
		if dead[pattern] {
			t.Errorf("the report prints %s as a page route with no typed link, and HomeView.gx has the link %s", pattern, link)
		}
	}
}
