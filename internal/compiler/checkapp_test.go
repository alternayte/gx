package compiler_test

import (
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

// TestREQ_AUT_19_CheckReportsGeneratorDiagnostics covers a diagnostic that
// only the generator can give: a value that cannot render as text is
// GX2013 in the check of an app, once, and not a silent pass
// (REQ-AUT-19, REQ-AUT-18).
func TestREQ_AUT_19_CheckReportsGeneratorDiagnostics(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":          moduleWithGx(t),
		"ui/card/Card.gx": "package card\n\nprops {\n  Tags map[string]int\n}\n\n<p>{p.Tags}</p>\n",
	})
	diags := compiler.CheckApp(dir, compiler.CheckOptions{})
	count := 0
	for _, d := range diags {
		if d.Code == compiler.CodeUnrenderable {
			count++
			if d.Line != 7 || d.Col == 0 {
				t.Fatalf("GX2013 position = %d:%d", d.Line, d.Col)
			}
		}
	}
	if count != 1 {
		t.Fatalf("CheckApp gave %d GX2013 diagnostics, want 1: %v", count, diags)
	}

	// A diagnostic of the check is not doubled by the generator.
	dir = writeTree(t, map[string]string{
		"go.mod":          moduleWithGx(t),
		"ui/card/Card.gx": "package card\n\nprops {\n  Title string\n}\n\n<article>{p.Title}</article>\n",
		"ui/card/Page.gx": "package card\n\n<Card />\n",
	})
	diags = compiler.CheckApp(dir, compiler.CheckOptions{})
	if len(diags) != 1 || diags[0].Code != compiler.CodeRequiredProp {
		t.Fatalf("CheckApp = %v, want one GX2001", diags)
	}
}

// TestREQ_CNT_02_CollectionWithoutComponents covers a collection that names
// no component: its frontmatter and its links are checked like those of a
// collection with a Components call (REQ-CNT-02, REQ-CNT-10).
func TestREQ_CNT_02_CollectionWithoutComponents(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":                moduleWithGx(t),
		"site/site.go":          "package site\n\nimport \"github.com/alternayte/gx\"\n\ntype Meta struct {\n\tTitle string `yaml:\"title\"`\n}\n\nvar Docs = gx.Collection[Meta](\"content/docs\")\n",
		"content/docs/start.md": "---\ntitle: Start\nauthor: Ada\n---\n\n# Start\n\nRead the [setup guide](/setup/) next.\n\n<site.Note>Text</site.Note>\n",
	})
	diags := compiler.CheckApp(dir, compiler.CheckOptions{})
	codes := map[string]int{}
	for _, d := range diags {
		codes[d.Code]++
	}
	for _, want := range []string{compiler.CodeContentFrontmatter, compiler.CodeContentLink, compiler.CodeContentComponent} {
		if codes[want] != 1 {
			t.Fatalf("%s reported %d times, want 1: %v", want, codes[want], diags)
		}
	}
}

// TestREQ_CNT_10_LinksUnderAPrefix covers the link check of a collection
// that the app mounts under a prefix: a link holds the prefix, a link to a
// page route of the app is valid, and a link to no page is GX8003
// (REQ-CNT-10).
func TestREQ_CNT_10_LinksUnderAPrefix(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":              moduleWithGx(t),
		"shop/route/route.go": "package route\n\nimport \"github.com/alternayte/gx\"\n\ntype Index struct {\n\tgx.Route `GET /shop`\n}\n\ntype Show struct {\n\tgx.Route `GET /shop/{id}`\n\tID int64\n}\n",
		"shop/IndexView.gx":   "package shop\n\n<h1>Shop</h1>\n",
		"shop/ShowView.gx":    "package shop\n\n<h1>Item</h1>\n",
		"shop/shop.go":        "package shop\n\nimport (\n\t\"github.com/alternayte/gx\"\n\t\"app/shop/route\"\n)\n\nvar IndexPage = gx.Page(func(c *gx.Ctx, in route.Index) (IndexViewProps, error) { return IndexViewProps{}, nil }, IndexView)\n\nvar ShowPage = gx.Page(func(c *gx.Ctx, in route.Show) (ShowViewProps, error) { return ShowViewProps{}, nil }, ShowView)\n\nvar Routes = gx.Collect(IndexPage, ShowPage)\n",
		"notes/notes.go":      "package notes\n\nimport \"github.com/alternayte/gx\"\n\ntype Meta struct {\n\tTitle string `yaml:\"title\"`\n}\n\nvar Notes = gx.Collection[Meta](\"content/notes\")\n\nfunc view(e gx.Entry[Meta]) gx.Node { return gx.Text(e.Meta.Title) }\n\nvar Routes = gx.Collect(gx.ContentEntries(Notes, view))\n",
		"main.go":             "package main\n\nimport (\n\t\"github.com/alternayte/gx\"\n\t\"app/notes\"\n\t\"app/shop\"\n)\n\nfunc main() {\n\tapp := gx.New(gx.Config{})\n\tapp.Group(\"/store\", shop.Routes)\n\tapp.Group(\"/notes\", notes.Routes)\n}\n",
		"content/notes/first.md": "---\ntitle: First\n---\n\n## Start\n\n" +
			"[second](/notes/second/)\n\n[heading](/notes/second/#end)\n\n[relative](second.md)\n\n" +
			"[the shop](/store/shop)\n\n[one item](/store/shop/7)\n\n" +
			"[no prefix](/second/)\n\n[no note](/notes/third/)\n\n[no heading](/notes/second/#none)\n\n[no page](/store/nowhere)\n",
		"content/notes/second.md": "---\ntitle: Second\n---\n\n## End\n\n[back](/notes/first/#start)\n",
	})
	var broken []string
	for _, d := range compiler.CheckWith(dir, compiler.CheckOptions{}) {
		if d.Code != compiler.CodeContentLink {
			t.Fatalf("an unexpected diagnostic: %s", d.String())
		}
		broken = append(broken, d.Msg)
	}
	want := []string{
		`link "/second/" points at no page`,
		`link "/notes/third/" points at no page`,
		`link "/notes/second/#none" points at no heading`,
		`link "/store/nowhere" points at no page`,
	}
	if len(broken) != len(want) {
		t.Fatalf("broken links = %q, want %q", broken, want)
	}
	for i := range want {
		if broken[i] != want[i] {
			t.Fatalf("broken links = %q, want %q", broken, want)
		}
	}
}
