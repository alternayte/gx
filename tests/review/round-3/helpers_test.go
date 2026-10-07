// Package round3_test holds the blocking findings of review round 3 for
// release 0.2.0 (SDD §16.3). Every test fails on the reviewed HEAD 71e3564.
package round3_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

// repoRoot returns the root of the gx module.
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
	root := repoRoot(t)
	all := map[string]string{
		"go.mod": "module app\n\ngo 1.25.0\n\nrequire github.com/alternayte/gx v0.0.0\n\nreplace github.com/alternayte/gx => " + filepath.ToSlash(root) + "\n",
	}
	if sum, err := os.ReadFile(filepath.Join(root, "go.sum")); err == nil {
		all["go.sum"] = string(sum)
	}
	for rel, content := range files {
		all[rel] = content
	}
	for rel, content := range all {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// generateInto runs the generator and writes its files into the module.
func generateInto(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	files, diags := compiler.Generate(dir)
	if len(diags) > 0 {
		t.Fatalf("Generate diagnostics: %v", diags)
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

// goRun runs the main package of a scratch module and returns its output.
func goRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("go", append([]string{"run"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
	out, _ := cmd.CombinedOutput()
	return string(out)
}
