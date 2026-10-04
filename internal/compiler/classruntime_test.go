package compiler_test

import (
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

// TestREQ_STY_11_RuntimeClasses covers GX5003: a class value built with
// fmt.Sprintf or concatenation is reported; static values and gx.Cx with
// constants stay clean (REQ-STY-11).
func TestREQ_STY_11_RuntimeClasses(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":          moduleWithGx(t),
		"ui/card/Card.gx": "package card\n\nimport (\n\t\"fmt\"\n\n\t\"github.com/alternayte/gx\"\n)\n\nprops {\n  N int\n  Extra string\n}\n\n<div class={fmt.Sprintf(\"p-%d\", p.N)}>a</div>\n<span class={\"p-4 \" + p.Extra}>b</span>\n<p class=\"text-sm\">c</p>\n<section class={gx.Cx(\"m-2\", \"m-4\")}>d</section>\n",
	})
	count := 0
	for _, d := range compiler.Check(dir) {
		if d.Code == compiler.CodeRuntimeClass {
			count++
			if !strings.Contains(d.Msg, "runtime") {
				t.Errorf("GX5003 message = %q", d.Msg)
			}
		}
	}
	if count != 2 {
		t.Fatalf("GX5003 count = %d, want 2", count)
	}
}
