package exporter_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alternayte/gx/internal/exporter"
	"github.com/alternayte/gx/internal/testbudget"
)

// TestNFR_11_HundredPageExport measures the export of a 100-page docs site
// with the search index included (NFR-11).
func TestNFR_11_HundredPageExport(t *testing.T) {
	mod := "module app\n\ngo 1.25.0\n\nrequire github.com/alternayte/gx v0.0.0\n\nreplace github.com/alternayte/gx => " + filepath.ToSlash(repoRoot(t)) + "\n"
	dir := writeTree(t, map[string]string{
		"go.mod":              mod,
		"products/content.go": "package products\n\nimport (\n\t\"github.com/alternayte/gx\"\n\t\"github.com/alternayte/gx/content\"\n)\n\ntype DocMeta struct {\n\tTitle string `yaml:\"title\"`\n}\n\nvar Docs = gx.Collection[DocMeta](\"content/docs\")\n\nfunc View(e gx.Entry[DocMeta]) gx.Node {\n\tnode, err := content.Body(e.Body)\n\tif err != nil {\n\t\treturn gx.Text(err.Error())\n\t}\n\treturn node\n}\n\nvar Routes = gx.Collect(gx.ContentEntries(Docs, View))\n\nfunc init() { content.Install() }\n",
		"main.go":             "package main\n\nimport (\n\t\"net/http\"\n\t\"os\"\n\n\t\"github.com/alternayte/gx\"\n\t\"app/products\"\n)\n\nfunc main() {\n\tapp := gx.New(gx.Config{})\n\tapp.Group(\"/\", products.Routes)\n\t_ = http.ListenAndServe(os.Getenv(\"GX_DEV_ADDR\"), app)\n}\n",
	})
	docs := filepath.Join(dir, "content", "docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100; i++ {
		body := fmt.Sprintf("---\ntitle: Page %d\n---\n\n# Page %d\n\n## Install\n\nSome words for the search index. Page %d.\n", i, i, i)
		name := filepath.Join(docs, fmt.Sprintf("page-%03d.md", i))
		if err := os.WriteFile(name, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out := filepath.Join(dir, "dist")
	start := time.Now()
	res, err := exporter.Export(context.Background(), exporter.Options{Dir: dir, Out: out, Main: "."})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	took := time.Since(start)
	t.Logf("NFR-11: 100-page export with search index %s", took)
	if len(res.Paths) != 100 {
		t.Fatalf("exported %d pages, want 100", len(res.Paths))
	}
	if _, err := os.Stat(filepath.Join(out, "pagefind", "pagefind.js")); err != nil {
		t.Fatalf("no search index: %v", err)
	}
	if budget := testbudget.Budget(10*time.Second, 6); took > budget {
		t.Fatalf("NFR-11: export took %s, want under %s on this machine", took, budget)
	}
	if strings.Contains(string(res.NotFound), "404") == false {
		t.Logf("404 body: %q", res.NotFound)
	}
}
