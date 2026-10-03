package compiler_test

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateRelativeRoot(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":          moduleWithGx(t),
		"ui/card/Card.gx": "package card\n\nprops {\n  Title string\n}\n\n<article>{p.Title}</article>\n",
	})
	t.Chdir(dir)
	files, diags := generateWithDot(t)
	if len(diags) > 0 {
		t.Fatalf("diagnostics = %v", diags)
	}
	card := string(files[filepath.Join(dir, "ui/card/Card_gx.go")])
	if !strings.Contains(card, "gx.Text(p.Title)") {
		t.Fatalf("relative root generated wrong code:\n%s", card)
	}
}
