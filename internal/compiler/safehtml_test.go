package compiler_test

import (
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

func TestSI_01_SafeHTMLOnlyWhenTrusted(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":          moduleWithGx(t),
		"ui/card/Card.gx": "package card\n\n<p>x</p>\n",
		"ui/card/card.go": "package card\n\nimport gx \"github.com/alternayte/gx\"\n\nvar userInput string\n\nvar a = gx.SafeHTML(userInput)\nvar b = gx.SafeHTML(\"<b>\")\nvar c = gx.SafeHTML(userInput) //gx:trusted escaped by sanitize.Comment\n",
	})
	diags := checkDir(t, dir)
	count := 0
	for _, d := range diags {
		if d.Code == compiler.CodeTrustedHTML {
			count++
			if d.Line != 7 {
				t.Errorf("GX7001 line = %d, want 7", d.Line)
			}
		}
	}
	if count != 1 {
		t.Fatalf("GX7001 count = %d, want 1: %v", count, diags)
	}
}
