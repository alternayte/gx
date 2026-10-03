package compiler_test

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

// moduleWithGx writes a go.mod for a test module that uses the gx checkout in
// this repository.
func moduleWithGx(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repo := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	return "module app\n\ngo 1.25.0\n\nrequire github.com/alternayte/gx v0.0.0\n\nreplace github.com/alternayte/gx => " + filepath.ToSlash(repo) + "\n"
}

func TestREQ_AUT_04_TypeErrorPosition(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":          moduleWithGx(t),
		"ui/card/Card.gx": "package card\n\nprops {\n  Title string\n}\n\n<article>{p.Titel}</article>\n",
	})
	d := diagWith(t, compiler.Check(dir), compiler.CodeType)
	if !strings.Contains(d.Msg, "Titel") {
		t.Fatalf("GX2000 message = %q", d.Msg)
	}
	if !strings.HasSuffix(d.File, "Card.gx") {
		t.Fatalf("GX2000 file = %q", d.File)
	}
	if d.Line != 7 || d.Col != 13 {
		t.Fatalf("GX2000 position = %d:%d, want 7:13", d.Line, d.Col)
	}
}

func TestREQ_AUT_04_ValidExpressions(t *testing.T) {
	src := "package card\n\nprops {\n  Title string\n  Items []int\n}\n\n<article>\n  {p.Title}\n  {len(p.Items)}\n  for i := 0; i < len(p.Items); i++ {\n    <i>{i}</i>\n  }\n  total := len(p.Items) + 1\n  <span>{total}</span>\n</article>\n"
	dir := writeTree(t, map[string]string{
		"go.mod":          moduleWithGx(t),
		"ui/card/Card.gx": src,
	})
	if diags := compiler.Check(dir); len(diags) != 0 {
		t.Fatalf("valid expressions: unexpected diagnostics %v", diags)
	}
}

func TestREQ_AUT_04_PropsTypeError(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":          moduleWithGx(t),
		"ui/card/Card.gx": "package card\n\nprops {\n  Title Missing\n}\n\n<article>{p.Title}</article>\n",
	})
	d := diagWith(t, compiler.Check(dir), compiler.CodeType)
	if !strings.Contains(d.Msg, "Missing") {
		t.Fatalf("GX2000 message = %q", d.Msg)
	}
	if d.Line != 4 || d.Col != 9 {
		t.Fatalf("GX2000 position = %d:%d, want 4:9", d.Line, d.Col)
	}
}

func TestREQ_AUT_04_AttributeExpressionError(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":          moduleWithGx(t),
		"ui/card/Card.gx": "package card\n\nprops {\n  Title string\n}\n\n<article class={pro}></article>\n",
	})
	d := diagWith(t, compiler.Check(dir), compiler.CodeType)
	if !strings.Contains(d.Msg, "pro") {
		t.Fatalf("GX2000 message = %q", d.Msg)
	}
	if d.Line != 7 || d.Col != 17 {
		t.Fatalf("GX2000 position = %d:%d, want 7:17", d.Line, d.Col)
	}
}
