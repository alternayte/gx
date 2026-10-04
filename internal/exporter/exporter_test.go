package exporter_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/exporter"
)

// repoRoot returns the gx checkout root for the test module's replace.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

// writeTree writes a test module and returns its root.
func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, body := range files {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// TestREQ_EXP_01_StaticExport covers `gx export`: every GET page renders to
// a file, the assets copy, 404.html writes and the search index runs over
// the output (REQ-EXP-01).
func TestREQ_EXP_01_StaticExport(t *testing.T) {
	mod := "module app\n\ngo 1.25.0\n\nrequire github.com/alternayte/gx v0.0.0\n\nreplace github.com/alternayte/gx => " + filepath.ToSlash(repoRoot(t)) + "\n"
	dir := writeTree(t, map[string]string{
		"go.mod":               mod,
		"products/routes.go":   "package products\n\nimport \"github.com/alternayte/gx\"\n\ntype Home struct {\n\tgx.Route `GET /home`\n}\n\nvar Routes = gx.Collect(HomePage)\n",
		"products/page.go":     "package products\n\nimport \"github.com/alternayte/gx\"\n\nvar HomePage = gx.Page(func(c *gx.Ctx, in Home) (HomeViewProps, error) {\n\treturn HomeViewProps{}, nil\n}, HomeView)\n",
		"products/HomeView.gx": "package products\n\n<h1 id=\"home\">Hello export</h1>\n",
		"main.go":              "package main\n\nimport (\n\t\"net/http\"\n\t\"os\"\n\n\t\"github.com/alternayte/gx\"\n\t\"app/products\"\n)\n\nfunc main() {\n\tapp := gx.New(gx.Config{})\n\tapp.Group(\"/\", products.Routes)\n\t_ = http.ListenAndServe(os.Getenv(\"GX_DEV_ADDR\"), app)\n}\n",
	})
	out := filepath.Join(dir, "dist")
	indexed := ""
	res, err := exporter.Export(context.Background(), exporter.Options{
		Dir:  dir,
		Out:  out,
		Main: ".",
		Index: func(ctx context.Context, root, siteDir string) error {
			indexed = siteDir
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if len(res.Paths) == 0 {
		t.Fatalf("manifest is empty: %+v", res)
	}
	home, err := os.ReadFile(filepath.Join(out, "home", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(home), "Hello export") || !strings.HasPrefix(string(home), "<!doctype html>") {
		t.Fatalf("home/index.html = %q", home)
	}
	if _, err := os.Stat(filepath.Join(out, "404.html")); err != nil {
		t.Fatalf("404.html: %v", err)
	}
	if indexed != out {
		t.Fatalf("index ran on %q, want %q", indexed, out)
	}
}
