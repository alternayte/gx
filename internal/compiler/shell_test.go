package compiler_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// copyRegistryItem copies the source of one registry item into dir.
func copyRegistryItem(t *testing.T, src, dst string) {
	t.Helper()
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
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

// TestREQ_CNT_06_DocsShell renders every fixture of the docs-shell block:
// the header, sidebar, table of contents, pagination, page meta, splash,
// 404 and the skip link (REQ-CNT-06).
func TestREQ_CNT_06_DocsShell(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(moduleWithGx(t)), 0o644); err != nil {
		t.Fatal(err)
	}
	copyRegistryItem(t, filepath.Join(registryDocs(t), "..", "docs-shell"), filepath.Join(dir, "ui", "shell"))
	files := writeGenerated(t, dir)

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
	_ = files

	if strings.Contains(out, " MISSING") {
		t.Errorf("a shell component has no fixtures:\n%s", out)
	}
	want := map[string]string{
		"Shell":        `class="gx-shell`,
		"Header":       `class="gx-header`,
		"Sidebar":      `class="gx-sidebar`,
		"SidebarGroup": `class="gx-nav-group`,
		"SidebarItem":  `class="gx-nav-link`,
		"Toc":          `class="gx-toc`,
		"Pagination":   `class="gx-pagination`,
		"PageMeta":     `class="gx-page-meta`,
		"Splash":       `class="gx-splash`,
		"NotFound":     `class="gx-not-found`,
		"ThemeSelect":  `data-gx-theme="dark"`,
		"SearchDialog": `data-gx-search`,
	}
	for comp, marker := range want {
		if !strings.Contains(out, "=== "+comp+"/") {
			t.Errorf("no rendered fixture for %s", comp)
		}
		if !strings.Contains(out, marker) {
			t.Errorf("rendered %s lacks %q", comp, marker)
		}
	}
	// The skip link, the header parts and the active sidebar item.
	for _, marker := range []string{
		`href="#gx-main"`,
		`class="gx-site-title`,
		`data-gx-version`,
		`data-gx-search-open`,
		`aria-current="page"`,
	} {
		if !strings.Contains(out, marker) {
			t.Errorf("shell output lacks %q", marker)
		}
	}
}
