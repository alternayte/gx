//go:build tailwindreal

package gxstyles_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/alternayte/gx"
	"github.com/alternayte/gx/internal/compiler"
	"github.com/alternayte/gx/internal/gxstyles"
)

// TestREQ_STY_02_ClassListInCSS covers the full styles pipeline: a class
// that appears only in a Go file of a .gx package lands in
// .gx/classes.txt and in the built CSS (REQ-STY-02). Run with
// `go test -tags tailwindreal` or `just tailwind-smoke`.
func TestREQ_STY_02_ClassListInCSS(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repo := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	dir := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module app\n\ngo 1.25.0\n\nrequire github.com/alternayte/gx v0.0.0\n\nreplace github.com/alternayte/gx => "+filepath.ToSlash(repo)+"\n")
	write("ui/Card.gx", "package ui\n\nprops {\n  Title string\n}\n\n<article class=\"rounded-xl\">{p.Title}</article>\n")
	write("ui/styles.go", "package ui\n\n// Extra is a class that appears only in Go code.\nvar Extra = \"tracking-widest\"\n")
	write("app/theme.css", gx.DefaultThemeCSS)

	files, diags := compiler.Generate(dir)
	if len(diags) > 0 {
		t.Fatalf("generate: %v", diags)
	}
	for path, src := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, src, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	okBuild, err := gxstyles.Build(context.Background(), dir, false)
	if err != nil {
		t.Fatalf("styles build: %v", err)
	}
	if !okBuild {
		t.Fatal("styles build skipped the app theme")
	}
	css, err := os.ReadFile(filepath.Join(dir, ".gx", "app.css"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"tracking-widest", "rounded-xl"} {
		if !strings.Contains(string(css), want) {
			t.Fatalf("built CSS lacks %q", want)
		}
	}
}
