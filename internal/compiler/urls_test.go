package compiler_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestREQ_RTE_04_TypeMismatch(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":               moduleWithGx(t),
		"products/routes.go":   routesGo,
		"products/ShowView.gx": "package products\n\nprops {\n  ID int64\n}\n\n<p>{p.ID}</p>\n",
		"products/page.go":     "package products\n\nimport \"github.com/alternayte/gx\"\n\nvar ShowPage = gx.Page(func(c *gx.Ctx, in Show) (int, error) {\n\treturn 0, nil\n}, ShowView)\n",
	})
	files := generateFiles(t, dir)
	for path, src := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, src, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("mismatched loader and view compiled")
	}
	if !strings.Contains(string(out), "ShowView") {
		t.Fatalf("compile error does not name the view:\n%s", out)
	}
}

func TestREQ_RTE_05_TypedLink(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":             moduleWithGx(t),
		"products/routes.go": routesGoNoRoutes,
		"products/Link.gx":   "package products\n\nprops {\n  ID   int64\n  Tab  string\n  Page int64\n}\n\n<a href={Show{ID: p.ID, Tab: p.Tab, Page: p.Page}}>go</a>\n",
		"products/link_test.go": "package products\n\nimport (\n\t\"strings\"\n\t\"testing\"\n\n\tgx \"github.com/alternayte/gx\"\n)\n\n" +
			"func TestLink(t *testing.T) {\n" +
			"\tgot := gx.String(Link(LinkProps{ID: 42, Tab: \"details\"}))\n" +
			"\tif !strings.Contains(got, `href=\"/products/42?tab=details\"`) {\n" +
			"\t\tt.Fatalf(\"details: %s\", got)\n" +
			"\t}\n" +
			"\tgot = gx.String(Link(LinkProps{ID: 7, Tab: \"overview\"}))\n" +
			"\tif !strings.Contains(got, `href=\"/products/7\"`) {\n" +
			"\t\tt.Fatalf(\"default: %s\", got)\n" +
			"\t}\n" +
			"}\n",
	})
	files := generateFiles(t, dir)
	route := string(files[filepath.Join(dir, "products/routes_gx.go")])
	for _, want := range []string{"func (in Show) URL() string", "url.PathEscape"} {
		if !strings.Contains(route, want) {
			t.Fatalf("routes_gx.go lacks %q:\n%s", want, route)
		}
	}
	link := string(files[filepath.Join(dir, "products/Link_gx.go")])
	if !strings.Contains(link, "Show{ID: p.ID, Tab: p.Tab, Page: p.Page}.URL()") {
		t.Fatalf("Link_gx.go does not call URL:\n%s", link)
	}
	for path, src := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, src, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go test: %v\n%s", err, out)
	}
}

func TestREQ_RTE_05_GxURLAllowed(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":          moduleWithGx(t),
		"ui/card/Card.gx": "package card\n\nimport gx \"github.com/alternayte/gx\"\n\n<a href={gx.URL(\"/docs\")}>docs</a>\n",
	})
	files := generateFiles(t, dir)
	card := string(files[filepath.Join(dir, "ui/card/Card_gx.go")])
	if !strings.Contains(card, "Kind: gx.AttrURL") {
		t.Fatalf("gx.URL is not a URL attribute:\n%s", card)
	}
	if diags := checkDir(t, dir); len(diags) != 0 {
		t.Fatalf("gx.URL rejected: %v", diags)
	}
}
