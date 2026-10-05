// Package round2_test holds the blocking findings of review round 2 for
// release 0.1.0 (SDD §16.3). Every test fails on the reviewed HEAD 680e6ef.
package round2_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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
func generateInto(t *testing.T, dir string) {
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
}

// goTest runs one test of a scratch module and returns the output. The
// env entries are added to the environment of the run.
func goTest(dir, name string, env ...string) string {
	cmd := exec.Command("go", "test", "-count=1", "-v", "-run", "^"+name+"$", "./...")
	cmd.Dir = dir
	cmd.Env = append(append(os.Environ(), "GOFLAGS=-mod=mod"), env...)
	out, _ := cmd.CombinedOutput()
	return string(out)
}

// innerPassed reports whether the scratch module output says that the
// named test passed.
func innerPassed(out, name string) bool {
	return strings.Contains(out, "--- PASS: "+name+" ")
}

// tail returns the last lines of the output of a scratch module run.
func tail(out string, n int) string {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
