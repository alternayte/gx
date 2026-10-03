package compiler_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

func TestREQ_RTE_18_BasePathCodegen(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":             moduleWithGx(t),
		"products/routes.go": routesGoNoRoutes,
	})
	files := generateFiles(t, dir)
	src := string(files[filepath.Join(dir, "products/routes_gx.go")])
	if !strings.Contains(src, "return gx.BasePath() + b.String()") {
		t.Fatalf("URL does not honour BasePath:\n%s", src)
	}
}

func TestREQ_RTE_20_RoutePackage(t *testing.T) {
	clean := writeTree(t, map[string]string{
		"go.mod":                  moduleWithGx(t),
		"products/route/route.go": "package route\n\nimport \"github.com/alternayte/gx\"\n\ntype Show struct {\n\tgx.Route `GET /products/{id}`\n\tID int64\n}\n",
	})
	if diags := checkDir(t, clean); len(diags) != 0 {
		t.Fatalf("clean route package: diagnostics = %v", diags)
	}

	bad := writeTree(t, map[string]string{
		"go.mod":                      moduleWithGx(t),
		"products/helpers/helpers.go": "package helpers\n\nfunc Help() {}\n",
		"products/route/route.go":     "package route\n\nimport (\n\t\"github.com/alternayte/gx\"\n\t\"app/products/helpers\"\n)\n\ntype Show struct {\n\tgx.Route `GET /products/{id}`\n\tID int64\n}\n\nfunc helper() { helpers.Help() }\n",
	})
	diags := checkDir(t, bad)
	count := 0
	for _, d := range diags {
		if d.Code == compiler.CodeRoutePkg {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("GX3005 count = %d, want 2 (import and func): %v", count, diags)
	}
}
