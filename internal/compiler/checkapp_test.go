package compiler_test

import (
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

// TestREQ_AUT_19_CheckReportsGeneratorDiagnostics covers a diagnostic that
// only the generator can give: a value that cannot render as text is
// GX2013 in the check of an app, once, and not a silent pass
// (REQ-AUT-19, REQ-AUT-18).
func TestREQ_AUT_19_CheckReportsGeneratorDiagnostics(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":          moduleWithGx(t),
		"ui/card/Card.gx": "package card\n\nprops {\n  Tags map[string]int\n}\n\n<p>{p.Tags}</p>\n",
	})
	diags := compiler.CheckApp(dir, compiler.CheckOptions{})
	count := 0
	for _, d := range diags {
		if d.Code == compiler.CodeUnrenderable {
			count++
			if d.Line != 7 || d.Col == 0 {
				t.Fatalf("GX2013 position = %d:%d", d.Line, d.Col)
			}
		}
	}
	if count != 1 {
		t.Fatalf("CheckApp gave %d GX2013 diagnostics, want 1: %v", count, diags)
	}

	// A diagnostic of the check is not doubled by the generator.
	dir = writeTree(t, map[string]string{
		"go.mod":          moduleWithGx(t),
		"ui/card/Card.gx": "package card\n\nprops {\n  Title string\n}\n\n<article>{p.Title}</article>\n",
		"ui/card/Page.gx": "package card\n\n<Card />\n",
	})
	diags = compiler.CheckApp(dir, compiler.CheckOptions{})
	if len(diags) != 1 || diags[0].Code != compiler.CodeRequiredProp {
		t.Fatalf("CheckApp = %v, want one GX2001", diags)
	}
}

// TestREQ_CNT_02_CollectionWithoutComponents covers a collection that names
// no component: its frontmatter and its links are checked like those of a
// collection with a Components call (REQ-CNT-02, REQ-CNT-10).
func TestREQ_CNT_02_CollectionWithoutComponents(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":                moduleWithGx(t),
		"site/site.go":          "package site\n\nimport \"github.com/alternayte/gx\"\n\ntype Meta struct {\n\tTitle string `yaml:\"title\"`\n}\n\nvar Docs = gx.Collection[Meta](\"content/docs\")\n",
		"content/docs/start.md": "---\ntitle: Start\nauthor: Ada\n---\n\n# Start\n\nRead the [setup guide](/setup/) next.\n\n<site.Note>Text</site.Note>\n",
	})
	diags := compiler.CheckApp(dir, compiler.CheckOptions{})
	codes := map[string]int{}
	for _, d := range diags {
		codes[d.Code]++
	}
	for _, want := range []string{compiler.CodeContentFrontmatter, compiler.CodeContentLink, compiler.CodeContentComponent} {
		if codes[want] != 1 {
			t.Fatalf("%s reported %d times, want 1: %v", want, codes[want], diags)
		}
	}
}
