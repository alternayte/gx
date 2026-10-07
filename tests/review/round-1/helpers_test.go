// Package round1_test holds the blocking findings of review round 1 for
// release 0.1.0 (SDD §16.3). Every test fails on the reviewed HEAD 8621ff5.
package round1_test

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/alternayte/gx"
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

// writeModule writes files into dir as a module that replaces gx with the
// repository under review.
func writeModule(t *testing.T, dir string, files map[string]string) {
	t.Helper()
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
}

// scratchModule returns a temp module with the files.
func scratchModule(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	writeModule(t, dir, files)
	return dir
}

// checkEach checks one module that holds the named components of package
// card and returns the diagnostics of each component.
func checkEach(t *testing.T, components map[string]string) map[string][]compiler.Diagnostic {
	t.Helper()
	files := map[string]string{}
	for name, src := range components {
		files["ui/card/"+name+".gx"] = src
	}
	diags := compiler.Check(scratchModule(t, files))
	out := map[string][]compiler.Diagnostic{}
	for name := range components {
		out[name] = diagsFor(diags, name+".gx")
	}
	return out
}

// diagsFor returns the diagnostics of one file, by base name.
func diagsFor(diags []compiler.Diagnostic, base string) []compiler.Diagnostic {
	var out []compiler.Diagnostic
	for _, d := range diags {
		if filepath.Base(d.File) == base {
			out = append(out, d)
		}
	}
	return out
}

// lineWith returns the last line of code that holds marker.
func lineWith(code, marker string) string {
	line := ""
	for _, l := range strings.Split(code, "\n") {
		if strings.Contains(l, marker) {
			line = strings.TrimSpace(l)
		}
	}
	return line
}

func hasCode(diags []compiler.Diagnostic, code string) bool {
	for _, d := range diags {
		if d.Code == code {
			return true
		}
	}
	return false
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

// goTestVerbose runs the tests of a scratch module and returns the output.
func goTestVerbose(dir string) string {
	cmd := exec.Command("go", "test", "-count=1", "-v", "./...")
	cmd.Dir = dir
	out, _ := cmd.CombinedOutput()
	return string(out)
}

// requireInnerPass fails t unless the scratch module output reports that
// the named test passed.
func requireInnerPass(t *testing.T, out, name string) {
	t.Helper()
	if strings.Contains(out, "--- PASS: "+name+" ") {
		return
	}
	// Keep the report short: the lines of the failing test only, or the
	// whole output when the module did not build.
	var keep []string
	on := false
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "=== RUN   "+name):
			on = true
		case strings.HasPrefix(line, "=== RUN"), strings.HasPrefix(line, "FAIL"), strings.HasPrefix(line, "ok "), strings.HasPrefix(line, "PASS"):
			on = false
		}
		if on {
			keep = append(keep, line)
		}
	}
	if len(keep) == 0 {
		keep = strings.Split(out, "\n")
	}
	if len(keep) > 40 {
		keep = keep[:40]
	}
	t.Fatalf("%s did not pass in the generated app:\n%s", name, strings.Join(keep, "\n"))
}

// fakeAdapter writes the HTML of every element patch, so a test can read
// what a navigation sends.
type fakeAdapter struct{}

func (fakeAdapter) Name() string                         { return "fake" }
func (fakeAdapter) Signals() bool                        { return true }
func (fakeAdapter) Runtime() gx.Node                     { return nil }
func (fakeAdapter) Assets() map[string][]byte            { return nil }
func (fakeAdapter) ReadSignals(*http.Request, any) error { return nil }
func (fakeAdapter) Invoke(method, url, scope string) gx.Attr {
	return gx.Attr{Key: "data-fake-on", Value: method + " " + url}
}

// On writes the event and the URL in a made-up syntax.
func (fakeAdapter) On(inv gx.Invocation) []gx.Attr {
	return []gx.Attr{{Key: "data-fake-on-" + inv.Event, Value: inv.Method + " " + inv.URL}}
}

func (fakeAdapter) Respond(w http.ResponseWriter, _ *http.Request, res *gx.Response) error {
	for _, p := range res.Patches {
		if ep, ok := p.(gx.ElementPatch); ok {
			_, _ = w.Write([]byte("PATCH " + ep.Target + " " + gx.String(ep.Node) + "\n"))
		}
	}
	return nil
}

// The generated-app tests share one scratch module and one go test run.
var (
	appOnce sync.Once
	appOut  string
	appErr  string
)
