package gxcli_test

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

// captureStderr runs fn with os.Stderr redirected and returns the output.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	fn()
	_ = w.Close()
	os.Stderr = old
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// TestREQ_TLS_03_LintCommand covers gx lint: it runs go vet and the Gx
// analyzers over every package and reports the findings with their codes
// (REQ-TLS-03).
func TestREQ_TLS_03_LintCommand(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repo := filepath.Clean(filepath.Join(filepath.Dir(file), ".."))
	mod := "module app\n\ngo 1.25.0\n\nrequire github.com/alternayte/gx v0.0.0\n\nreplace github.com/alternayte/gx => " + filepath.ToSlash(repo) + "\n"

	dir := t.TempDir()
	for rel, content := range map[string]string{
		"go.mod":         mod,
		"cart/cart.go":   "package cart\n\nimport gx \"github.com/alternayte/gx\"\n\nvar userInput string\n\nvar a = gx.SafeHTML(userInput)\n",
		"route/route.go": "package route\n\nfunc Nope() {}\n",
		"ui/styles.go":   "package ui\n\nimport \"github.com/alternayte/gx\"\n\ntype V string\n\nconst (\n\tA V = \"a\"\n\tB V = \"b\"\n)\n\nvar m = gx.Enum[V]{A: \"x\"}\n",
	} {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	out := captureStderr(t, func() {
		if code := gxcli.Main([]string{"lint", dir}); code == 0 {
			t.Error("lint accepted a module with findings")
		}
	})
	for _, want := range []string{"GX7001", "GX3005", "GX5001", "cart.go", "route.go"} {
		if !strings.Contains(out, want) {
			t.Fatalf("lint output lacks %q:\n%s", want, out)
		}
	}
}

// lspFrame frames one JSON-RPC message for the stdio test.
func lspFrame(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return []byte(fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(data), data))
}

// readLSPFrames parses every framed message in data.
func readLSPFrames(t *testing.T, data []byte) []map[string]any {
	t.Helper()
	var out []map[string]any
	for len(data) > 0 {
		idx := strings.Index(string(data), "\r\n\r\n")
		if idx < 0 {
			break
		}
		var length int
		if _, err := fmt.Sscanf(string(data[:idx]), "Content-Length: %d", &length); err != nil {
			t.Fatal(err)
		}
		body := data[idx+4 : idx+4+length]
		var msg map[string]any
		if err := json.Unmarshal(body, &msg); err != nil {
			t.Fatal(err)
		}
		out = append(out, msg)
		data = data[idx+4+length:]
	}
	return out
}

// TestREQ_TLS_04_StdioLSP covers the `gx lsp` command: standard LSP over
// stdio with no editor-specific extensions (REQ-TLS-04).
func TestREQ_TLS_04_StdioLSP(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repo := filepath.Clean(filepath.Join(filepath.Dir(file), ".."))
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "ui/card"), 0o755); err != nil {
		t.Fatal(err)
	}
	card := filepath.Join(dir, "ui/card/Card.gx")
	bad := "package card\n\n<article>{p.Titel}</article>\n"
	if err := os.WriteFile(card, []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	mod := "module app\n\ngo 1.25.0\n\nrequire github.com/alternayte/gx v0.0.0\n\nreplace github.com/alternayte/gx => " + filepath.ToSlash(repo) + "\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(mod), 0o644); err != nil {
		t.Fatal(err)
	}

	inR, inW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldIn, oldOut := os.Stdin, os.Stdout
	os.Stdin, os.Stdout = inR, outW

	uri := "file://" + filepath.ToSlash(card)
	frames := [][]byte{
		lspFrame(t, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{"processId": nil, "rootUri": nil, "capabilities": map[string]any{}}}),
		lspFrame(t, map[string]any{"jsonrpc": "2.0", "method": "initialized", "params": map[string]any{}}),
		lspFrame(t, map[string]any{"jsonrpc": "2.0", "method": "textDocument/didOpen", "params": map[string]any{"textDocument": map[string]any{"uri": uri, "languageId": "gx", "version": 1, "text": bad}}}),
		lspFrame(t, map[string]any{"jsonrpc": "2.0", "id": 2, "method": "shutdown", "params": nil}),
		lspFrame(t, map[string]any{"jsonrpc": "2.0", "method": "exit", "params": nil}),
	}
	go func() {
		for _, f := range frames {
			_, _ = inW.Write(f)
		}
		_ = inW.Close()
	}()

	code := gxcli.Main([]string{"lsp", "--root", dir})
	_ = outW.Close()
	os.Stdin, os.Stdout = oldIn, oldOut
	data, err := io.ReadAll(outR)
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatalf("lsp exit = %d", code)
	}

	msgs := readLSPFrames(t, data)
	var capabilities, diagnostics int
	for _, msg := range msgs {
		switch msg["method"] {
		case "textDocument/publishDiagnostics":
			diagnostics++
		}
		if result, ok := msg["result"].(map[string]any); ok {
			if caps, ok := result["capabilities"].(map[string]any); ok && caps["completionProvider"] != nil {
				capabilities++
			}
		}
	}
	if capabilities != 1 {
		t.Fatalf("initialize result lacks capabilities: %v", msgs)
	}
	if diagnostics == 0 {
		t.Fatalf("no diagnostics published over stdio: %v", msgs)
	}
}
