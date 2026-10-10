// Package round8_test holds the blocking findings of review round 8 for
// release 0.4.0 (SDD §16.3, §18). Every test fails on the reviewed HEAD
// 3cbd354.
package round8_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

// scratchModule returns a temp module with the files. The module replaces
// gx with the repository under review.
func scratchModule(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	all := map[string]string{
		"go.mod": "module app\n\ngo 1.25.0\n\nrequire github.com/alternayte/gx v0.0.0\n\nreplace github.com/alternayte/gx => " + filepath.ToSlash(repoRoot(t)) + "\n",
	}
	for rel, content := range files {
		all[rel] = content
	}
	writeFiles(t, dir, all)
	return dir
}

func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// generateModule writes the generated files of the module, as gx generate
// does, and returns them.
func generateModule(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	files, diags := compiler.NewSession().Generate(dir)
	if compiler.Failed(diags) {
		t.Fatalf("the fixture does not compile: %v", diags)
	}
	for path, src := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, src, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return files
}

// importBlock returns the import declaration for the imports that
// propgen.Fixture names. An entry is an import path. An entry that holds a
// quote is an import spec, with a name before the path.
func importBlock(imports []string, more ...string) string {
	var b strings.Builder
	b.WriteString("import (\n")
	for _, path := range more {
		b.WriteString("\t" + strconv.Quote(path) + "\n")
	}
	for _, imp := range imports {
		if strings.Contains(imp, `"`) {
			b.WriteString("\t" + imp + "\n")
			continue
		}
		b.WriteString("\t" + strconv.Quote(imp) + "\n")
	}
	b.WriteString(")\n")
	return b.String()
}

// runProgram runs main.go as a program of its own module, which has only
// the standard library. It returns the output, or the output of the
// compiler with ok false when the program does not compile.
func runProgram(t *testing.T, source string) (out string, ok bool) {
	t.Helper()
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"go.mod": "module fixture\n\ngo 1.25.0\n", "main.go": source})
	cmd := exec.Command("go", "run", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod")
	data, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(data)), err == nil
}

// buildWithFile builds the package in pkgDir of the repository with one
// more file, which is not on the disk: the go command reads it from an
// overlay. It returns the output of the compiler and whether the build
// passed.
func buildWithFile(t *testing.T, pkgDir, name, source string) (out string, ok bool) {
	t.Helper()
	tmp := t.TempDir()
	src := filepath.Join(tmp, name)
	if err := os.WriteFile(src, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	overlay, _ := json.Marshal(map[string]any{"Replace": map[string]string{filepath.Join(pkgDir, name): src}})
	overlayPath := filepath.Join(tmp, "overlay.json")
	if err := os.WriteFile(overlayPath, overlay, 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "build", "-overlay", overlayPath, "-o", os.DevNull, ".")
	cmd.Dir = pkgDir
	data, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(data)), err == nil
}
