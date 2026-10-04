package analyze_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/analyze"
)

func writeModule(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repo := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	files["go.mod"] = "module app\n\ngo 1.25.0\n\nrequire github.com/alternayte/gx v0.0.0\n\nreplace github.com/alternayte/gx => " + filepath.ToSlash(repo) + "\n"
	for rel, content := range files {
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

// TestREQ_STY_11_LintRuntimeClass covers the gxclassruntime analyzer: a
// dynamic gx.Cx argument is GX5003, a constant one is clean (REQ-STY-11).
func TestREQ_STY_11_LintRuntimeClass(t *testing.T) {
	dir := writeModule(t, map[string]string{
		"ui/styles.go": "package ui\n\nimport \"github.com/alternayte/gx\"\n\nvar dynamic string\n\nvar a = gx.Cx(\"p-4\", dynamic)\nvar b = gx.Cx(\"p-4\", \"m-2\")\n",
	})
	findings, err := analyze.Lint(dir)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, f := range findings {
		if f.Code == "GX5003" {
			count++
			if !strings.Contains(f.Message, "runtime") {
				t.Errorf("message = %q", f.Message)
			}
		}
	}
	if count != 1 {
		t.Fatalf("GX5003 findings = %d, want 1: %+v", count, findings)
	}
}
