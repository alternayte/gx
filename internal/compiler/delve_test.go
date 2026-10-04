package compiler_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

// TestREQ_TLS_08_DelveBreakpoint covers the debugging contract: the
// generated code maps the loop body back to the .gx line and the Delve
// smoke script breaks there and reads p and the loop variable
// (REQ-TLS-08).
func TestREQ_TLS_08_DelveBreakpoint(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":          moduleWithGx(t),
		"ui/card/Card.gx": "package card\n\nprops {\n  Title string\n  Items []string\n}\n\n<article>\n  for _, it := range p.Items {\n    <p>{it}</p>\n  }\n</article>\n",
	})
	files, diags := compiler.Generate(dir)
	if len(diags) > 0 {
		t.Fatalf("generate: %v", diags)
	}
	generated := string(files[filepath.Join(dir, "ui/card/Card_gx.go")])
	if !strings.Contains(generated, "//line Card.gx:10") {
		t.Fatalf("the loop body is not mapped to Card.gx:10:\n%s", generated)
	}

	// The smoke script breaks on that line and reads p and it.
	script := readScript(t, "delve-smoke.sh")
	for _, want := range []string{
		"break Card.gx:10",
		"print p.Title",
		"print it",
		"locals",
		"all=-N -l",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("delve-smoke.sh lacks %q", want)
		}
	}
}

func readScript(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "scripts", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
