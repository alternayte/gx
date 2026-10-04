package compiler_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

// TestREQ_STY_02_ClassList covers the class list: static classes, class:
// directives and the class strings of Go files of packages that hold .gx
// files land in .gx/classes.txt (REQ-STY-02).
func TestREQ_STY_02_ClassList(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":            moduleWithGx(t),
		"ui/card/styles.go": "package card\n\nimport \"github.com/alternayte/gx\"\n\nfunc pick(raised bool) string {\n\treturn gx.Cx(\"inline-flex items-center\", map[bool]string{true: \"shadow-lg\"}[raised])\n}\n",
		"ui/card/Card.gx":   "package card\n\nprops {\n  Raised bool\n}\n\n<article class=\"rounded-xl p-4\" class:shadow-lg={p.Raised}>\n  <span class=\"text-sm\">x</span>\n</article>\n",
	})
	files, diags := compiler.Generate(dir)
	if len(diags) > 0 {
		t.Fatalf("generate: %v", diags)
	}
	data, ok := files[filepath.Join(dir, ".gx", "classes.txt")]
	if !ok {
		t.Fatalf("no class list in %v", files)
	}
	got := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		if line != "" {
			got[line] = true
		}
	}
	for _, want := range []string{"rounded-xl", "p-4", "shadow-lg", "text-sm", "inline-flex", "items-center"} {
		if !got[want] {
			t.Fatalf("class list lacks %q:\n%s", want, data)
		}
	}
}
