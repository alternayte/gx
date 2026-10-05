package analyze_test

import (
	"fmt"
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
// gx.Cx argument built with fmt.Sprintf, fmt.Sprint, strings.Join or a
// concatenation with a non-constant is GX5003. A constant, a prop, a
// gx.Enum lookup, a plain variable and a function result are clean: their
// class text is a static string somewhere that Tailwind reads (REQ-STY-11,
// B-004).
func TestREQ_STY_11_LintRuntimeClass(t *testing.T) {
	dir := writeModule(t, map[string]string{
		"ui/styles.go": `package ui

import (
	"fmt"
	"strings"

	"github.com/alternayte/gx"
)

type Variant string

const Primary Variant = "primary"

var variant = gx.Enum[Variant]{Primary: "bg-primary"}

type Props struct {
	Class   string
	Variant Variant
}

var size = "4"
var parts = []string{"p-4", "m-2"}

func classes() string { return "p-4" }

// Built at runtime: Tailwind cannot see these.
var a = gx.Cx("p-4", fmt.Sprintf("m-%s", size))
var b = gx.Cx("p-4", "m-"+size)
var c = gx.Cx(strings.Join(parts, " "))
var d = gx.Cx("p-4", (fmt.Sprint("m-", size)))

// Static somewhere: clean.
var e = gx.Cx("p-4", "m-2")
var f = gx.Cx("p-" + "4")
var g = gx.Cx("p-4", size)
var h = gx.Cx(classes(), parts[0])

func (p Props) class() string {
	return gx.Cx("inline-flex", variant[p.Variant], p.Class)
}
`,
	})
	findings, err := analyze.Lint(dir)
	if err != nil {
		t.Fatal(err)
	}
	var lines []int
	for _, f := range findings {
		if f.Code == "GX5003" {
			lines = append(lines, f.Line)
			if !strings.Contains(f.Message, "runtime") {
				t.Errorf("message = %q", f.Message)
			}
		}
	}
	if got := fmt.Sprint(lines); got != "[27 28 29 30]" {
		t.Fatalf("GX5003 lines = %s, want the four runtime cases [27 28 29 30]: %+v", got, findings)
	}
}
