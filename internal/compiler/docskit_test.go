package compiler_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

// registryDocs returns the repo path of the docs kit registry item.
func registryDocs(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "registry", "docs")
}

// copyDocsKit copies every source file of the docs kit into dir/ui/docs.
func copyDocsKit(t *testing.T, dir string) {
	t.Helper()
	src := registryDocs(t)
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "ui", "docs")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, "_gx.go") || !(strings.HasSuffix(name, ".gx") || strings.HasSuffix(name, ".go")) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(src, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dst, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// runModule runs one go command in a test module. The module has no
// go.sum, so the command may add missing entries.
func runModule(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

// TestREQ_CNT_05_DocsKit renders every fixture of the docs kit registry
// item: every kit component has fixtures, and each renders its own markup
// (REQ-CNT-05).
func TestREQ_CNT_05_DocsKit(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(moduleWithGx(t)), 0o644); err != nil {
		t.Fatal(err)
	}
	copyDocsKit(t, dir)
	_ = writeGenerated(t, dir)

	mainSrc := `package main

import (
	"fmt"

	gx "github.com/alternayte/gx"
	"app/gxdev_gallery"
)

func main() {
	for _, f := range gxdev_gallery.Fixtures() {
		if f.Missing {
			fmt.Printf("=== %s MISSING\n", f.Component)
			continue
		}
		fmt.Printf("=== %s/%s\n%s\n", f.Component, f.Name, gx.String(f.Node()))
	}
}
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(mainSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	out := runModule(t, dir, "run", "-tags", "gxdev", ".")

	want := map[string]string{
		"Aside":           `class="gx-aside`,
		"Badge":           `<span class="inline-flex items-center rounded-full border`,
		"Card":            `class="gx-card`,
		"CardGrid":        `class="gx-card-grid`,
		"Code":            `class="gx-code"`,
		"FileTree":        `class="gx-file-tree`,
		"FileTreeItemRow": `class="gx-file-tree-item"`,
		"Hero":            `class="gx-hero`,
		"Icon":            `<svg`,
		"LinkButton":      `class="gx-link-button`,
		"LinkCard":        `class="gx-link-card`,
		"Steps":           `class="gx-steps`,
		"TabItem":         `class="gx-tab"`,
		"Tabs":            `class="gx-tabs`,
	}
	if strings.Contains(out, " MISSING") {
		t.Errorf("a kit component has no fixtures:\n%s", out)
	}
	for comp, marker := range want {
		if !strings.Contains(out, "=== "+comp+"/") {
			t.Errorf("no rendered fixture for %s", comp)
		}
		if !strings.Contains(out, marker) {
			t.Errorf("rendered %s lacks %q", comp, marker)
		}
	}
	// The Code fixture is highlighted Go.
	for _, marker := range []string{`data-lang="go"`, "tok-keyword"} {
		if !strings.Contains(out, marker) {
			t.Errorf("Code fixture lacks %q", marker)
		}
	}
	// The tabs fixture renders buttons and panels that the runtime wires.
	for _, marker := range []string{`data-gx-tab="Postgres"`, `data-gx-tab-panel`} {
		if !strings.Contains(out, marker) {
			t.Errorf("Tabs fixture lacks %q", marker)
		}
	}
}

// TestREQ_CNT_05_CodeFile covers the build-time resolution of gx.CodeFile:
// the selected lines land in the generated code and render, and a missing
// file or line range fails the build (REQ-CNT-05).
func TestREQ_CNT_05_CodeFile(t *testing.T) {
	src := "line one\nline two\nline three\nline four\n"
	codeGx := "package docs\n\nimport \"github.com/alternayte/gx/content\"\n\nprops {\n  Code gx.Code\n}\n\n{content.Code(p.Code, content.CodeOptions{})}\n"
	pageGx := "package docs\n\n<Code code={gx.CodeFile(\"content/sample.go\", \"2-3\")} />\n"
	dir := writeTree(t, map[string]string{
		"go.mod":               moduleWithGx(t),
		"content/sample.go":    src,
		"ui/docs/Code.gx":      codeGx,
		"ui/docs/Page.gx":      pageGx,
		"ui/docs/page_test.go": pageTest,
	})
	files := writeGenerated(t, dir)
	generated := string(files[filepath.Join(dir, "ui/docs/Page_gx.go")])
	for _, want := range []string{`File: "content/sample.go"`, `Lang: "go"`, `Source: "line two\nline three\n"`} {
		if !strings.Contains(generated, want) {
			t.Fatalf("Page_gx.go lacks %q:\n%s", want, generated)
		}
	}
	out := runModule(t, dir, "test", "./ui/docs/")
	if !strings.Contains(out, "ok") {
		t.Fatalf("code page test did not pass:\n%s", out)
	}

	// A line range past the end of the file is GX8004.
	bad := writeTree(t, map[string]string{
		"go.mod":            moduleWithGx(t),
		"content/sample.go": src,
		"ui/docs/Code.gx":   codeGx,
		"ui/docs/Page.gx":   "package docs\n\n<Code code={gx.CodeFile(\"content/sample.go\", \"2-99\")} />\n",
	})
	diags := compiler.Check(bad)
	if !hasCode(diags, compiler.CodeCodeFile) {
		t.Fatalf("out-of-range lines: diagnostics = %v, want GX8004", diags)
	}

	// A missing file is GX8004 too.
	missing := writeTree(t, map[string]string{
		"go.mod":          moduleWithGx(t),
		"ui/docs/Code.gx": codeGx,
		"ui/docs/Page.gx": "package docs\n\n<Code code={gx.CodeFile(\"content/nope.txt\", \"1\")} />\n",
	})
	diags = compiler.Check(missing)
	if !hasCode(diags, compiler.CodeCodeFile) {
		t.Fatalf("missing file: diagnostics = %v, want GX8004", diags)
	}
}

// pageTest renders the Page component and checks the resolved code block.
const pageTest = `package docs

import (
	"strings"
	"testing"

	gx "github.com/alternayte/gx"
)

func TestPage(t *testing.T) {
	out := gx.String(Page(PageProps{}))
	for _, want := range []string{"two", "three", "gx-code", "data-lang=\"go\""} {
		if !strings.Contains(out, want) {
			t.Fatalf("output = %q, want %q", out, want)
		}
	}
	for _, bad := range []string{"one", "four"} {
		if strings.Contains(out, bad) {
			t.Fatalf("output = %q, want only the selected lines", out)
		}
	}
}
`
