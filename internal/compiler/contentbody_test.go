package compiler_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestREQ_CNT_03_ContentBodyRender covers the generated body renderer:
// component tags in Markdown become typed calls with nested children and
// prose, and fenced code keeps its tags as text (REQ-CNT-03).
func TestREQ_CNT_03_ContentBodyRender(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":                moduleWithGx(t),
		"docs/Aside.gx":         "package docs\n\nprops {\n  Kind     Kind = Note\n  Children gx.Node\n}\n\n<aside class={kindClass[p.Kind]}>{p.Children}</aside>\n",
		"docs/aside.go":         "package docs\n\nimport \"github.com/alternayte/gx\"\n\ntype Kind string\n\nconst Tip Kind = \"tip\"\nconst Note Kind = \"note\"\n\nvar kindClass = gx.Enum[Kind]{Note: \"gx-aside\", Tip: \"gx-aside gx-aside-tip\"}\n",
		"docs/Tabs.gx":          "package docs\n\nprops {\n  Sync     string = \"\"\n  Children gx.Node\n}\n\n<div class=\"gx-tabs\" data-sync={p.Sync}>{p.Children}</div>\n",
		"docs/TabItem.gx":       "package docs\n\nprops {\n  Label    string\n  Children gx.Node\n}\n\n<div class=\"gx-tab\" data-label={p.Label}>{p.Children}</div>\n",
		"docs/content.go":       "package docs\n\nimport (\n\t\"github.com/alternayte/gx\"\n\t\"github.com/alternayte/gx/content\"\n)\n\ntype DocMeta struct {\n\tTitle string `yaml:\"title\"`\n}\n\nvar Docs = gx.Collection[DocMeta](\"content/docs\").Components(Aside, Tabs, TabItem)\n\nfunc View(e gx.Entry[DocMeta]) gx.Node { return DocsBody(e) }\n\nvar _ = content.Install\n",
		"content/docs/start.md": "# Start\n\nintro\n\n<docs.Aside kind=\"tip\">Tip body</docs.Aside>\n\n```gx\n<docs.Tabs><docs.TabItem label=\"A\">x</docs.TabItem></docs.Tabs>\n```\n\n<docs.Tabs sync=\"db\">\n  <docs.TabItem label=\"A\">Body A</docs.TabItem>\n  <docs.TabItem label=\"B\">Body B</docs.TabItem>\n</docs.Tabs>\n\noutro\n",
		"docs/render_test.go":   "package docs\n\nimport (\n\t\"strings\"\n\t\"testing\"\n\n\tgx \"github.com/alternayte/gx\"\n)\n\nfunc TestRender(t *testing.T) {\n\tout := gx.String(View(gx.Entry[DocMeta]{Slug: \"start\"}))\n\tfor _, want := range []string{\"<h1 id=\\\"start\\\">Start\", \"<p>intro</p>\", `class=\"gx-aside gx-aside-tip\"`, \"Tip body\", `data-label=\"A\"`, \"Body A\", \"Body B\", \"<p>outro</p>\"} {\n\t\tif !strings.Contains(out, want) {\n\t\t\tt.Fatalf(\"output lacks %q:\\n%s\", want, out)\n\t\t}\n\t}\n\tif strings.Contains(out, `data-label=\"A\">x<`) {\n\t\tt.Fatalf(\"the fenced tag rendered:\\n%s\", out)\n\t}\n\torder := []string{\"intro\", \"Tip body\", \"Body A\", \"outro\"}\n\tlast := -1\n\tfor _, part := range order {\n\t\tat := strings.Index(out, part)\n\t\tif at < last {\n\t\t\tt.Fatalf(\"out of order %q:\\n%s\", part, out)\n\t\t}\n\t\tlast = at\n\t}\n}\n",
	})
	files := writeGenerated(t, dir)
	for path, src := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, src, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, ok := files[filepath.Join(dir, "docs", "gxcontent_gx.go")]; !ok {
		t.Fatalf("no generated body file in %v", files)
	}
	cmd := exec.Command("go", "test", "./docs/")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go test: %v\n%s", err, out)
	}
}
