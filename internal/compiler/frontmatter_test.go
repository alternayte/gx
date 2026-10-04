package compiler_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

// TestREQ_CNT_02_Frontmatter covers GX8001: malformed YAML, an unknown field
// and a wrong value kind each report the file and line; valid frontmatter is
// clean (REQ-CNT-02).
func TestREQ_CNT_02_Frontmatter(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":                  moduleWithGx(t),
		"docs/content.go":         "package docs\n\nimport \"github.com/alternayte/gx\"\n\ntype DocMeta struct {\n\tTitle string `yaml:\"title\"`\n\tOrder int    `yaml:\"order\"`\n}\n\nvar Docs = gx.Collection[DocMeta](\"content/docs\").Components()\n",
		"content/docs/ok.md":      "---\ntitle: Start\norder: 1\n---\n\n# Start\n",
		"content/docs/kind.md":    "---\ntitle: K\norder: nope\n---\n\n# K\n",
		"content/docs/unknown.md": "---\ntitle: U\nnope: 1\n---\n\n# U\n",
		"content/docs/syntax.md":  "---\ntitle: [unclosed\n---\n\n# S\n",
	})
	byFile := map[string]compiler.Diagnostic{}
	for _, d := range compiler.Check(dir) {
		if d.Code != compiler.CodeContentFrontmatter {
			continue
		}
		base := filepath.Base(d.File)
		byFile[base] = d
	}
	if got, ok := byFile["ok.md"]; ok {
		t.Fatalf("valid frontmatter reported: %+v", got)
	}
	kind, ok := byFile["kind.md"]
	if !ok || kind.Line != 3 || !strings.Contains(kind.Msg, "order") {
		t.Fatalf("kind diagnostic = %+v", kind)
	}
	unknown, ok := byFile["unknown.md"]
	if !ok || unknown.Line != 3 || !strings.Contains(unknown.Msg, "nope") {
		t.Fatalf("unknown diagnostic = %+v", unknown)
	}
	if _, ok := byFile["syntax.md"]; !ok {
		t.Fatalf("no syntax diagnostic: %v", byFile)
	}
}
