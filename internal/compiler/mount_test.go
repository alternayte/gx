package compiler_test

import (
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

func TestREQ_RTE_06_UnmountedRoute(t *testing.T) {
	unmounted := "package products\n\nimport \"github.com/alternayte/gx\"\n\nvar ShowPage = gx.Page(func(c *gx.Ctx, in struct{}) (int, error) {\n\treturn 0, nil\n}, func(i int) gx.Node { return nil })\n"
	dir := writeTree(t, map[string]string{
		"go.mod":           moduleWithGx(t),
		"products/page.go": unmounted,
	})
	if !hasCode(checkDir(t, dir), compiler.CodeUnmounted) {
		t.Fatalf("unmounted page: want GX3003")
	}

	dir = writeTree(t, map[string]string{
		"go.mod":           moduleWithGx(t),
		"products/page.go": unmounted + "\nvar Routes = gx.Collect(ShowPage)\n",
	})
	if diags := checkDir(t, dir); len(diags) != 0 {
		t.Fatalf("mounted page: unexpected diagnostics %v", diags)
	}
}

func TestREQ_RTE_07_DuplicatePattern(t *testing.T) {
	src := "package products\n\nimport \"github.com/alternayte/gx\"\n\ntype Show struct {\n\tgx.Route `GET /products/{id}`\n\tID int64\n}\n\ntype Read struct {\n\tgx.Route `GET /products/{id}`\n\tID int64\n}\n"
	dir := writeTree(t, map[string]string{
		"go.mod":             moduleWithGx(t),
		"products/routes.go": src,
	})
	diags := checkDir(t, dir)
	if !hasCode(diags, compiler.CodeDuplicate) {
		t.Fatalf("duplicate pattern: diagnostics = %v, want GX3004", diags)
	}
}
