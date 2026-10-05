package compiler_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

// buildGenerated writes the generated files of dir and runs a go command
// in it.
func buildGenerated(t *testing.T, dir string, args ...string) {
	t.Helper()
	for path, src := range generateFiles(t, dir) {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, src, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go %v: %v\n%s", args, err, out)
	}
}

// TestREQ_RTE_05_ExactRootLink covers a typed link to a pattern that ends
// with the ServeMux end anchor: "GET /{$}" links to "/" and "GET /docs/{$}"
// links to "/docs/" (REQ-RTE-05).
func TestREQ_RTE_05_ExactRootLink(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":           moduleWithGx(t),
		"home/route/r.go":  "package route\n\nimport \"github.com/alternayte/gx\"\n\ntype Home struct {\n\tgx.Route `GET /{$}`\n}\n\ntype Docs struct {\n\tgx.Route `GET /docs/{$}`\n}\n",
		"home/Nav.gx":      "package home\n\nimport \"app/home/route\"\n\n<nav><a href={route.Home{}}>Home</a><a href={route.Docs{}}>Docs</a></nav>\n",
		"home/nav_test.go": "package home\n\nimport (\n\t\"strings\"\n\t\"testing\"\n\n\tgx \"github.com/alternayte/gx\"\n)\n\nfunc TestNav(t *testing.T) {\n\twant := `<nav><a href=\"/\" data-gx-active=\"page\">Home</a><a href=\"/docs/\" data-gx-active=\"page\">Docs</a></nav>`\n\tif got := strings.TrimSpace(gx.String(Nav(NavProps{}))); got != want {\n\t\tt.Fatalf(\"got  %s\\nwant %s\", got, want)\n\t}\n}\n",
	})
	buildGenerated(t, dir, "test", "./...")
}

// TestREQ_FRM_03_RouteFileImports covers the imports of a generated route
// file: a form with only string fields next to plain routes compiles, and
// so does every other mix of field kinds (REQ-FRM-03).
func TestREQ_FRM_03_RouteFileImports(t *testing.T) {
	files := map[string]string{"go.mod": moduleWithGx(t)}
	for name, routes := range map[string]string{
		"string form":   "type Join struct {\n\tgx.Route `POST /join`\n\tEmail string\n}\n\nfunc (in *Join) Rules() gx.Rules {\n\treturn gx.Rules{gx.Field(&in.Email, gx.Required)}\n}\n\ntype Add struct {\n\tgx.Route `POST /cart/add`\n}\n",
		"int form":      "type Join struct {\n\tgx.Route `POST /join/age`\n\tAge int\n}\n\nfunc (in *Join) Rules() gx.Rules {\n\treturn gx.Rules{gx.Field(&in.Age, gx.Min(1))}\n}\n",
		"no fields":     "type Home struct {\n\tgx.Route `GET /`\n}\n",
		"string path":   "type Show struct {\n\tgx.Route `GET /p/{slug}`\n\tSlug string\n}\n",
		"int path":      "type Show struct {\n\tgx.Route `GET /q/{id}`\n\tID int64\n\tTab string `query:\"tab\"`\n}\n",
		"action fields": "type Set struct {\n\tgx.Route `POST /set`\n\tQty int\n\tNote string\n}\n",
	} {
		files["slice"+strconv.Itoa(len(files))+"/route/r.go"] = "package route\n\nimport \"github.com/alternayte/gx\"\n\n// " + name + "\n" + routes
	}
	// One module and one build keep the test light: every case is its own
	// route package.
	buildGenerated(t, writeTree(t, files), "build", "./...")
}
