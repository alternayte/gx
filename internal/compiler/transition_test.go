package compiler_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

// TestREQ_STY_07_TransitionCodegen covers transition={T(k)}: the generated
// Go renders the sanitized view-transition style (REQ-STY-07).
func TestREQ_STY_07_TransitionCodegen(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":            moduleWithGx(t),
		"ui/card/styles.go": "package card\n\nimport \"github.com/alternayte/gx\"\n\nvar Hero = gx.Transition[int64](\"product-image\")\n",
		"ui/card/Card.gx":   "package card\n\nimport \"github.com/alternayte/gx\"\n\nprops {\n  ID int64\n}\n\n<img transition={Hero(p.ID)} src=\"/a.png\" alt=\"a\" />\n",
	})
	files, diags := compiler.Generate(dir)
	if len(diags) > 0 {
		t.Fatalf("generate: %v", diags)
	}
	generated := string(files[filepath.Join(dir, "ui/card/Card_gx.go")])
	if !strings.Contains(generated, "gx.TransitionStyle(Hero(p.ID))") {
		t.Fatalf("generated code lacks the transition style:\n%s", generated)
	}
}

// TestREQ_STY_08_TransitionDuplicates covers GX5002: the same transition
// value twice in one template is an error, different keys are clean
// (REQ-STY-08).
func TestREQ_STY_08_TransitionDuplicates(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":            moduleWithGx(t),
		"ui/card/styles.go": "package card\n\nimport \"github.com/alternayte/gx\"\n\nvar Hero = gx.Transition[int64](\"hero\")\n",
		"ui/card/Card.gx":   "package card\n\nimport \"github.com/alternayte/gx\"\n\nprops {\n  A int64\n  B int64\n}\n\n<div>\n  <img transition={Hero(p.A)} src=\"/a.png\" alt=\"a\" />\n  <img transition={Hero(p.A)} src=\"/b.png\" alt=\"b\" />\n</div>\n",
	})
	count := 0
	for _, d := range compiler.Check(dir) {
		if d.Code == compiler.CodeTransition {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("GX5002 count = %d, want 1", count)
	}

	clean := writeTree(t, map[string]string{
		"go.mod":            moduleWithGx(t),
		"ui/card/styles.go": "package card\n\nimport \"github.com/alternayte/gx\"\n\nvar Hero = gx.Transition[int64](\"hero\")\n",
		"ui/card/Card.gx":   "package card\n\nimport \"github.com/alternayte/gx\"\n\nprops {\n  A int64\n  B int64\n}\n\n<div>\n  <img transition={Hero(p.A)} src=\"/a.png\" alt=\"a\" />\n  <img transition={Hero(p.B)} src=\"/b.png\" alt=\"b\" />\n</div>\n",
	})
	for _, d := range compiler.Check(clean) {
		if d.Code == compiler.CodeTransition {
			t.Fatalf("different keys reported: %v", d)
		}
	}
}
