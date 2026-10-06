package gxcli_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/alternayte/gx/gxcli"
)

// islandApp writes an app with one island.
func islandApp(t *testing.T, island string) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	repo := filepath.Clean(filepath.Join(filepath.Dir(file), ".."))
	dir := t.TempDir()
	for rel, content := range map[string]string{
		"go.mod":        "module app\n\ngo 1.25.0\n\nrequire github.com/alternayte/gx v0.0.0\n\nreplace github.com/alternayte/gx => " + filepath.ToSlash(repo) + "\n",
		"dash/props.go": "package dash\n\ntype ChartProps struct {\n\tTitle string `json:\"title\"`\n}\n",
		"dash/Chart.ts": island,
	} {
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

// `gx check` fails on a type error in an island and prints it as a
// diagnostic, also with --json (REQ-ISL-08, REQ-AI-02).
func TestREQ_ISL_08_CheckCommand(t *testing.T) {
	dir := islandApp(t, "import type { Mount } from \"./Chart.props\";\n\nconst mount: Mount = (el, { title }) => {\n  el.textContent = title.length.trim();\n};\n\nexport default mount;\n")
	if code := gxcli.Main([]string{"generate", dir}); code != 0 {
		t.Fatalf("gx generate = %d", code)
	}
	out, code := captureStdout(t, func() int { return gxcli.Main([]string{"check", "--json", dir}) })
	if code != 1 {
		t.Fatalf("gx check --json = %d, want 1\n%s", code, out)
	}
	var diags []struct {
		Code, File, Message, Doc string
		Line, Column             int
	}
	if err := json.Unmarshal([]byte(out), &diags); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if len(diags) != 1 || diags[0].Code != "GX6005" || diags[0].Line != 4 || diags[0].Doc != "/errors/GX6005" || !strings.Contains(diags[0].Message, "TS2339") {
		t.Fatalf("diagnostics = %+v", diags)
	}
	fixed := "import type { Mount } from \"./Chart.props\";\n\nconst mount: Mount = (el, { title }) => {\n  el.textContent = title.trim();\n};\n\nexport default mount;\n"
	if err := os.WriteFile(filepath.Join(dir, "dash", "Chart.ts"), []byte(fixed), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := gxcli.Main([]string{"check", dir}); code != 0 {
		t.Fatalf("gx check with a correct island = %d", code)
	}
}

// `gx pin` vendors a package from the CDN of --cdn, and `gx build` bundles
// the island that imports it (REQ-ISL-07).
func TestREQ_ISL_07_PinCommand(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/npm/chartlib@1.2.0/+esm" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte("export default (s)=>\"PINNED:\"+s;\n"))
	}))
	defer srv.Close()
	dir := islandApp(t, "import chart from \"chartlib\";\n\nexport default (el: HTMLElement) => {\n  el.textContent = chart(\"x\");\n};\n")
	if code := gxcli.Main([]string{"pin", dir}); code == 0 {
		t.Fatal("gx pin with no package passed")
	}
	out, code := captureStdout(t, func() int { return gxcli.Main([]string{"pin", "--cdn", srv.URL, "chartlib@1.2.0", dir}) })
	if code != 0 || !strings.Contains(out, "pinned chartlib@1.2.0: 1 files in js/vendor") {
		t.Fatalf("gx pin = %d\n%s", code, out)
	}
	lock, err := os.ReadFile(filepath.Join(dir, "gx.lock"))
	if err != nil || !strings.Contains(string(lock), `"js/vendor/chartlib@1.2.0.js": "sha256-`) {
		t.Fatalf("gx.lock = %s, %v", lock, err)
	}
	if code := gxcli.Main([]string{"pin", "--cdn", srv.URL, "other@1.0.0", dir}); code != 1 {
		t.Fatalf("gx pin of a missing package = %d, want 1", code)
	}
}
