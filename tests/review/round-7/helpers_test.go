// Package round7_test holds the blocking findings of review round 7 for
// release 0.4.0 (SDD §16.3, §18). Every test fails on the reviewed HEAD
// b1ca912.
package round7_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

// checkModule writes the generated files of the module and returns the
// diagnostics of gx check as "code file:line" texts.
func checkModule(t *testing.T, dir string) ([]string, []compiler.Diagnostic) {
	t.Helper()
	files, diags := compiler.Generate(dir)
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
	var out []string
	diags = compiler.CheckApp(dir, compiler.CheckOptions{})
	for _, d := range diags {
		out = append(out, d.Code+" "+filepath.Base(d.File)+":"+itoa(d.Line))
	}
	return out, diags
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{byte('0' + n%10)}, b...)
	}
	return string(b)
}

// viewFindings checks one component with the body and returns the findings.
// The body starts at line 10 of View.gx.
func viewFindings(t *testing.T, body string, more map[string]string) []string {
	t.Helper()
	files := map[string]string{
		"ui/x/View.gx": "package x\n\nprops {\n  On    bool\n  ID    string\n  Attrs gx.Attrs = nil\n  Items []string\n}\n\n" + body + "\n",
	}
	for k, v := range more {
		files[k] = v
	}
	got, _ := checkModule(t, scratchModule(t, files))
	return got
}

func joined(s []string) string { return strings.Join(s, ", ") }

// has reports whether the findings hold one with the code.
func has(findings []string, code string) bool {
	for _, f := range findings {
		if strings.HasPrefix(f, code+" ") {
			return true
		}
	}
	return false
}

// runBun writes a script to a temp directory and runs it in Bun. The text
// %JS% in the script is the directory runtime/js of the repository.
func runBun(t *testing.T, script string) string {
	t.Helper()
	if _, err := exec.LookPath("bun"); err != nil {
		t.Fatalf("bun is not on PATH: %v", err)
	}
	script = strings.ReplaceAll(script, "%JS%", filepath.ToSlash(filepath.Join(repoRoot(t), "runtime", "js")))
	path := filepath.Join(t.TempDir(), "script.mjs")
	if err := os.WriteFile(path, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("bun", "run", path).CombinedOutput()
	if err != nil {
		t.Fatalf("bun: %v\n%s", err, out)
	}
	return strings.TrimSpace(string(out))
}
