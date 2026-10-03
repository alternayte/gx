package gxcli_test

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/alternayte/gx/gxcli"
)

func TestREQ_AUT_17_FmtCommand(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Card.gx")
	unformatted := []byte("package card\n\n\n<p>x</p>\n")
	if err := os.WriteFile(path, unformatted, 0o644); err != nil {
		t.Fatal(err)
	}

	if code := gxcli.Main([]string{"fmt", "--check", path}); code == 0 {
		t.Fatal("--check accepted an unformatted file")
	}

	if code := gxcli.Main([]string{"fmt", path}); code != 0 {
		t.Fatalf("fmt exit code = %d", code)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) == string(unformatted) {
		t.Fatal("fmt did not rewrite the file")
	}
	if code := gxcli.Main([]string{"fmt", path}); code != 0 {
		t.Fatalf("second fmt exit code = %d", code)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("fmt is not idempotent:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
	if code := gxcli.Main([]string{"fmt", "--check", path}); code != 0 {
		t.Fatalf("--check rejected a formatted file: exit %d", code)
	}

	if code := gxcli.Main([]string{"fmt", filepath.Join(dir, "Missing.gx")}); code == 0 {
		t.Fatal("fmt accepted a missing file")
	}
	if code := gxcli.Main([]string{"frobnicate"}); code == 0 {
		t.Fatal("unknown command exited zero")
	}
}

func TestREQ_AUT_18_StaleCheck(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repo := filepath.Clean(filepath.Join(filepath.Dir(file), ".."))
	mod := "module app\n\ngo 1.25.0\n\nrequire github.com/alternayte/gx v0.0.0\n\nreplace github.com/alternayte/gx => " + filepath.ToSlash(repo) + "\n"

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "ui/card"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(rel, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", mod)
	write("ui/card/Card.gx", "package card\n\nprops {\n  Title string\n}\n\n<article>{p.Title}</article>\n")

	if code := gxcli.Main([]string{"check", dir}); code == 0 {
		t.Fatal("check accepted missing generated code")
	}
	if code := gxcli.Main([]string{"generate", dir}); code != 0 {
		t.Fatalf("generate exit = %d", code)
	}
	if code := gxcli.Main([]string{"check", dir}); code != 0 {
		t.Fatal("check rejected fresh generated code")
	}
	write("ui/card/Card.gx", "package card\n\nprops {\n  Title string\n}\n\n<article><h1>{p.Title}</h1></article>\n")
	if code := gxcli.Main([]string{"check", dir}); code == 0 {
		t.Fatal("check accepted stale generated code")
	}
}

func TestREQ_AUT_19_JSONOutput(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repo := filepath.Clean(filepath.Join(filepath.Dir(file), ".."))
	mod := "module app\n\ngo 1.25.0\n\nrequire github.com/alternayte/gx v0.0.0\n\nreplace github.com/alternayte/gx => " + filepath.ToSlash(repo) + "\n"

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "ui/card"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(rel, content string) error {
		return os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), []byte(content), 0o644)
	}
	if err := write("go.mod", mod); err != nil {
		t.Fatal(err)
	}
	if err := write("ui/card/Card.gx", "package card\n\nprops {\n  Title string\n}\n\n<article>{p.Title}</article>\n"); err != nil {
		t.Fatal(err)
	}
	if err := write("ui/card/Page.gx", "package card\n\n<Card />\n"); err != nil {
		t.Fatal(err)
	}

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	code := gxcli.Main([]string{"check", "--json", dir})
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdout = old
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if code != 1 {
		t.Fatalf("check --json exit = %d, want 1\n%s", code, data)
	}
	var got []struct {
		Code string `json:"code"`
		File string `json:"file"`
		Line int    `json:"line"`
		Doc  string `json:"doc"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("bad JSON: %v\n%s", err, data)
	}
	if len(got) == 0 || got[0].Code != "GX2001" || got[0].Doc != "/errors/GX2001" {
		t.Fatalf("JSON diagnostics = %+v", got)
	}
}

func TestREQ_RTE_14_RoutesJSON(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repo := filepath.Clean(filepath.Join(filepath.Dir(file), ".."))
	mod := "module app\n\ngo 1.25.0\n\nrequire github.com/alternayte/gx v0.0.0\n\nreplace github.com/alternayte/gx => " + filepath.ToSlash(repo) + "\n"

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "products"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod":             mod,
		"products/routes.go": "package products\n\nimport \"github.com/alternayte/gx\"\n\ntype Show struct {\n\tgx.Route `GET /products/{id}`\n\tID int64\n}\n\nvar Routes = gx.Collect(ShowPage)\n",
		"products/page.go":   "package products\n\nimport \"github.com/alternayte/gx\"\n\nvar ShowPage = gx.Page(func(c *gx.Ctx, in Show) (int, error) { return 0, nil }, func(i int) gx.Node { return gx.Text(\"x\") })\n",
	}
	for rel, content := range files {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	code := gxcli.Main([]string{"routes", "--json", dir})
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdout = old
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatalf("routes --json exit = %d\n%s", code, data)
	}
	var got []map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("bad JSON: %v\n%s", err, data)
	}
	if len(got) != 1 || got[0]["pattern"] != "GET /products/{id}" || got[0]["page"] != "products.ShowPage" {
		t.Fatalf("routes JSON = %s", data)
	}
}
