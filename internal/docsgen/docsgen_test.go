package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

// repoRoot returns the repository root.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

// registryImport is the import prefix of the registry source. `gx add`
// rewrites it to the app module, and so does the test.
const registryImport = "github.com/alternayte/gx/registry/"

// installRegistry copies the source files of every item into dir/ui, as
// `gx add` does: a component tag resolves only inside the app module.
func installRegistry(t *testing.T, reg *registry, dir string) {
	t.Helper()
	for _, it := range reg.Items {
		entries, err := os.ReadDir(it.Dir)
		if err != nil {
			t.Fatal(err)
		}
		dst := filepath.Join(dir, "ui", filepath.Base(it.Dir))
		if err := os.MkdirAll(dst, 0o755); err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			name := e.Name()
			// The source files of an item: .gx, .go and the .ts file of
			// an island. The generated files stay out.
			island := strings.HasSuffix(name, ".ts") && !strings.HasSuffix(name, ".props.ts")
			if e.IsDir() || strings.HasSuffix(name, "_gx.go") || strings.HasSuffix(name, "_test.go") || !(strings.HasSuffix(name, ".gx") || strings.HasSuffix(name, ".go") || island) {
				continue
			}
			data, err := os.ReadFile(filepath.Join(it.Dir, name))
			if err != nil {
				t.Fatal(err)
			}
			data = []byte(strings.ReplaceAll(string(data), registryImport, "app/ui/"))
			if err := os.WriteFile(filepath.Join(dst, name), data, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// TestREQ_DOC_04_GeneratedSnippetsCompile installs the official registry in
// an app, writes the example code of every fixture as a .gx file of that
// app, checks the files with the real parser and checker, and builds the Go
// code they generate (REQ-DOC-04). No fixture falls back to an expression
// that names the fixture.
func TestREQ_DOC_04_GeneratedSnippetsCompile(t *testing.T) {
	root := repoRoot(t)
	reg, err := loadRegistry(root)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	mod := "module app\n\ngo 1.25.0\n\nrequire github.com/alternayte/gx v0.0.0\n\nreplace github.com/alternayte/gx => " + filepath.ToSlash(root) + "\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(mod), 0o644); err != nil {
		t.Fatal(err)
	}
	installRegistry(t, reg, dir)
	pkg := filepath.Join(dir, "snippets")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	names := map[string]string{} // file name -> fixture
	tags := 0
	for _, p := range buildPages(reg) {
		for _, ex := range p.Examples {
			for _, part := range ex.Parts {
				if part.Snippet.Tag {
					tags++
				}
				// Every example shows the code a user writes: the tag,
				// or the call of a component that is a Go function.
				if !part.Snippet.Tag && !part.Snippet.Call {
					t.Errorf("%s %s/%s falls back to its fixture name: %s", p.Item.Name, part.Comp.Name, part.Fixture.Name, part.Snippet.Why)
				}
				name := fmt.Sprintf("S%04d.gx", len(names))
				names[name] = p.Item.Name + " " + part.Comp.Name + "/" + part.Fixture.Name
				var src strings.Builder
				src.WriteString("package snippets\n\n")
				var quals []string
				for qual := range part.Snippet.Imports {
					if qual != "gx" {
						quals = append(quals, qual)
					}
				}
				sort.Strings(quals)
				if len(quals) > 0 {
					src.WriteString("import (\n")
					for _, qual := range quals {
						fmt.Fprintf(&src, "  %s %q\n", qual, strings.Replace(part.Snippet.Imports[qual], registryImport, "app/ui/", 1))
					}
					src.WriteString(")\n\n")
				}
				src.WriteString(part.Snippet.Code + "\n")
				if err := os.WriteFile(filepath.Join(pkg, name), []byte(src.String()), 0o644); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	if len(names) == 0 || tags == 0 {
		t.Fatalf("the registry gives %d snippets, %d of them tags", len(names), tags)
	}

	t.Setenv("GOFLAGS", "-mod=mod")
	tidy := exec.Command("go", "mod", "tidy")
	tidy.Dir = dir
	if out, err := tidy.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy: %v\n%s", err, out)
	}
	files, diags := compiler.Generate(dir)
	if len(diags) > 0 {
		d := diags[0]
		src, _ := os.ReadFile(d.File)
		t.Fatalf("the snippet of %s does not compile: %s:%d:%d: %s %s\n%s",
			names[filepath.Base(d.File)], filepath.Base(d.File), d.Line, d.Col, d.Code, d.Msg, src)
	}
	for path, src := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, src, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	build := exec.Command("go", "build", "./...")
	build.Dir = dir
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("the generated Go of the snippets does not build: %v\n%s", err, out)
	}
}
