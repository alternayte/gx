package compiler_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

const routesGo = "package products\n" +
	"\n" +
	"import \"github.com/alternayte/gx\"\n" +
	"\n" +
	"type Show struct {\n" +
	"\tgx.Route `GET /products/{id}`\n" +
	"\tID   int64\n" +
	"\tTab  string `query:\"tab\" default:\"overview\"`\n" +
	"\tPage int64  `query:\"page\"`\n" +
	"}\n" +
	"\n" +
	"var Routes = gx.Collect(ShowPage)\n"

const routesPageGo = "package products\n" +
	"\n" +
	"import \"github.com/alternayte/gx\"\n" +
	"\n" +
	"var ShowPage = gx.Page(func(c *gx.Ctx, in Show) (ShowViewProps, error) {\n" +
	"\treturn ShowViewProps{ID: in.ID, Tab: in.Tab, Page: in.Page}, nil\n" +
	"}, ShowView)\n"

func TestREQ_RTE_01_RouteMatches(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":               moduleWithGx(t),
		"products/routes.go":   routesGo,
		"products/page.go":     routesPageGo,
		"products/ShowView.gx": "package products\n\nprops {\n  ID   int64\n  Tab  string\n  Page int64\n}\n\n<p>id={p.ID} tab={p.Tab} page={p.Page}</p>",
		"products/route_test.go": "package products\n\nimport (\n\t\"net/http/httptest\"\n\t\"testing\"\n\n\t\"github.com/alternayte/gx\"\n)\n\n" +
			"func TestRoute(t *testing.T) {\n" +
			"\tapp := gx.New(gx.Config{})\n" +
			"\tapp.Group(\"/\", Routes)\n" +
			"\tget := func(target string) *httptest.ResponseRecorder {\n" +
			"\t\treq := httptest.NewRequest(\"GET\", target, nil)\n" +
			"\t\trec := httptest.NewRecorder()\n" +
			"\t\tapp.ServeHTTP(rec, req)\n" +
			"\t\treturn rec\n" +
			"\t}\n" +
			"\tif rec := get(\"/products/42?tab=details\"); rec.Code != 200 || rec.Body.String() != \"<p>id=42 tab=details page=0</p>\" {\n" +
			"\t\tt.Fatalf(\"details: %d %q\", rec.Code, rec.Body.String())\n" +
			"\t}\n" +
			"\tif rec := get(\"/products/42\"); rec.Code != 200 || rec.Body.String() != \"<p>id=42 tab=overview page=0</p>\" {\n" +
			"\t\tt.Fatalf(\"default: %d %q\", rec.Code, rec.Body.String())\n" +
			"\t}\n" +
			"\tif rec := get(\"/products/42?page=abc\"); rec.Code != 400 {\n" +
			"\t\tt.Fatalf(\"bad query: status %d\", rec.Code)\n" +
			"\t}\n" +
			"}\n",
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
	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go test: %v\n%s", err, out)
	}
}

func TestREQ_RTE_02_PathMismatch(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":             moduleWithGx(t),
		"products/routes.go": "package products\n\nimport \"github.com/alternayte/gx\"\n\ntype Show struct {\n\tgx.Route `GET /products/{id}`\n\tSKU string\n}\n",
	})
	diags := checkDir(t, dir)
	if !hasCode(diags, compiler.CodePathVar) {
		t.Fatalf("missing field for {id}: diagnostics = %v, want GX3001", diags)
	}

	dir = writeTree(t, map[string]string{
		"go.mod":             moduleWithGx(t),
		"products/routes.go": "package products\n\nimport \"github.com/alternayte/gx\"\n\ntype Show struct {\n\tgx.Route `GET /products/{id}`\n\tID  int64\n\tSKU string `path:\"sku\"`\n}\n",
	})
	diags = checkDir(t, dir)
	if !hasCode(diags, compiler.CodePathField) {
		t.Fatalf("path tag without variable: diagnostics = %v, want GX3002", diags)
	}
}

func TestREQ_RTE_03_QueryDecode(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":             moduleWithGx(t),
		"products/routes.go": "package products\n\nimport \"github.com/alternayte/gx\"\n\ntype Search struct {\n\tgx.Route `GET /search`\n\tQ    string `query:\"q\"`\n\tPage int64  `query:\"page\" default:\"1\"`\n}\n",
	})
	files := generateFiles(t, dir)
	src := string(files[filepath.Join(dir, "products/routes_gx.go")])
	for _, want := range []string{
		`func (Search) Pattern() string { return "GET /search" }`,
		`in.Page = int64(x)`,
		`if in.Page == 0 {`,
		`in.Page = 1`,
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("routes_gx.go lacks %q:\n%s", want, src)
		}
	}
}
