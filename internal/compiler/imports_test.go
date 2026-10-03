package compiler_test

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateDropsUnusedImports(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":              moduleWithGx(t),
		"ui/button/Button.gx": "package button\n\nprops {\n  Label string\n}\n\n<button>{p.Label}</button>\n",
		"ui/card/Card.gx":     "package card\n\nimport \"app/ui/button\"\n\nprops {\n  Title string\n}\n\n<article>{p.Title}</article>\n",
	})
	files := generateFiles(t, dir)
	card := string(files[filepath.Join(dir, "ui/card/Card_gx.go")])
	if strings.Contains(card, "app/ui/button") {
		t.Fatalf("unused import survived codegen:\n%s", card)
	}
}
