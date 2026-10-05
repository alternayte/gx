package gxcli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/alternayte/gx/gxcli"
)

// scratchModule writes files into a temp module that replaces gx with this
// repository, and returns its symlink-free root.
func scratchModule(t *testing.T, files map[string]string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repo := filepath.Clean(filepath.Join(filepath.Dir(file), ".."))
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
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

// snapshot compares got with a golden file. GX_UPDATE_GOLDEN=1 rewrites it.
func snapshot(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if os.Getenv("GX_UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bytes.ReplaceAll(want, []byte("\r\n"), []byte("\n")), []byte(got)) {
		t.Fatalf("%s differs from the snapshot:\n%s", name, got)
	}
}

// TestREQ_AI_02_DiagnosticsJSON covers the machine output of diagnostics:
// `gx check --json` and `gx lint --json` print code, location, message, doc
// link and fix for every diagnostic (REQ-AI-02).
func TestREQ_AI_02_DiagnosticsJSON(t *testing.T) {
	checkDir := scratchModule(t, map[string]string{
		"ui/card/Card.gx":    "package card\n\nprops {\n  Title string\n}\n\n<article>{p.Title}</article>\n",
		"ui/card/Counter.gx": "package card\n\nsignals {\n  Qty int\n}\n\n<p text={$Qty}></p>\n",
		"ui/card/Page.gx":    "package card\n\n<div>\n  <Card />\n  <Crad title=\"x\" />\n  <Card title=\"x\" tilte=\"y\" />\n</div>\n",
	})
	lintDir := scratchModule(t, map[string]string{
		"cart/cart.go":   "package cart\n\nimport gx \"github.com/alternayte/gx\"\n\nvar userInput string\n\nvar a = gx.SafeHTML(userInput)\n",
		"route/route.go": "package route\n\nfunc Nope() {}\n",
		"ui/styles.go":   "package ui\n\nimport \"github.com/alternayte/gx\"\n\ntype V string\n\nconst (\n\tA V = \"a\"\n\tB V = \"b\"\n)\n\nvar m = gx.Enum[V]{A: \"x\"}\n",
	})

	for _, tc := range []struct {
		golden string
		dir    string
		args   []string
		codes  []string
	}{
		{"ai02_check.golden.json", checkDir, []string{"check", "--json", checkDir}, []string{"GX2001", "GX2002", "GX2003", "GX2014"}},
		{"ai02_lint.golden.json", lintDir, []string{"lint", "--json", lintDir}, []string{"GX7001", "GX3005", "GX5001"}},
	} {
		out, code := captureStdout(t, func() int { return gxcli.Main(tc.args) })
		if code != 1 {
			t.Fatalf("gx %s exit = %d, want 1\n%s", strings.Join(tc.args[:2], " "), code, out)
		}
		var got []map[string]any
		if err := json.Unmarshal([]byte(out), &got); err != nil {
			t.Fatalf("gx %s: bad JSON: %v\n%s", tc.args[0], err, out)
		}
		seen := map[string]bool{}
		for _, d := range got {
			code, _ := d["code"].(string)
			seen[code] = true
			for _, key := range []string{"code", "file", "line", "column", "message", "doc"} {
				if _, ok := d[key]; !ok {
					t.Fatalf("gx %s: diagnostic lacks %q: %v", tc.args[0], key, d)
				}
			}
			if d["doc"] != "/errors/"+code {
				t.Fatalf("gx %s: doc link = %v", tc.args[0], d["doc"])
			}
			if file, _ := d["file"].(string); !strings.HasPrefix(filepath.ToSlash(file), filepath.ToSlash(tc.dir)) {
				t.Fatalf("gx %s: file %q is not under the module", tc.args[0], file)
			}
		}
		if tc.args[0] == "check" {
			fixes := 0
			for _, d := range got {
				if fix, _ := d["fix"].(string); fix != "" {
					fixes++
				}
			}
			if fixes == 0 {
				t.Fatalf("gx check: no diagnostic carries a fix:\n%s", out)
			}
		}
		for _, want := range tc.codes {
			if !seen[want] {
				t.Fatalf("gx %s: no %s in\n%s", tc.args[0], want, out)
			}
		}
		normal := strings.ReplaceAll(out, strings.ReplaceAll(filepath.ToSlash(tc.dir), "/", `\/`), "<root>")
		normal = strings.ReplaceAll(normal, filepath.ToSlash(tc.dir), "<root>")
		normal = strings.ReplaceAll(normal, `\\`, "/")
		snapshot(t, tc.golden, normal)
	}
}
