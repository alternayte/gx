package gxcli_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/gx/gxcli"
	"github.com/alternayte/gx/internal/registry"
)

// registryItem is one item of the test registry source.
type registryItem struct {
	name    string
	version string
	deps    []string
	files   map[string]string
	target  string
}

// writeRegistryItem writes one item folder into a registry source.
func writeRegistryItem(t *testing.T, src string, item registryItem) {
	t.Helper()
	version := item.version
	if version == "" {
		version = "0.1.0"
	}
	dir := filepath.Join(src, item.name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	var files []map[string]string
	for file := range item.files {
		files = append(files, map[string]string{"path": file, "target": item.target})
	}
	manifest := map[string]any{
		"name": item.name, "version": version, "description": item.name + " test item",
		"kind": "component", "files": files,
	}
	if len(item.deps) > 0 {
		manifest["registryDependencies"] = item.deps
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "gx-item.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "USAGE.md"), []byte("# "+item.name+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for file, content := range item.files {
		if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// fixture is a registry source and its published output.
type fixture struct {
	src string
	out string
}

// build republishes the source.
func (f *fixture) build(t *testing.T) {
	t.Helper()
	if _, err := registry.Build(f.src, f.out); err != nil {
		t.Fatal(err)
	}
}

const dialogV1 = "package dialog\n\nprops {\n  Title string\n}\n\n<dialog>{p.Title}</dialog>\n"

// registryFixture builds a published registry with a button item and a
// dialog item that depends on it (REQ-REG-02).
func registryFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{src: t.TempDir(), out: t.TempDir()}
	writeRegistryItem(t, f.src, registryItem{
		name:   "button",
		files:  map[string]string{"Button.gx": "package button\n\nprops {\n  Label string\n}\n\n<button>{p.Label}</button>\n"},
		target: "ui/button/Button.gx",
	})
	writeRegistryItem(t, f.src, registryItem{
		name:   "dialog",
		deps:   []string{"button"},
		files:  map[string]string{"Dialog.gx": dialogV1},
		target: "ui/dialog/Dialog.gx",
	})
	f.build(t)
	return f
}

// appWithRegistry writes a gx.toml that points at the registry.
func appWithRegistry(t *testing.T, published string) string {
	t.Helper()
	app := t.TempDir()
	toml := "[registry]\nurl = \"" + published + "\"\n"
	if err := os.WriteFile(filepath.Join(app, "gx.toml"), []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	return app
}

// captureStdout runs fn and returns what it wrote to stdout.
func captureStdout(t *testing.T, fn func() int) (string, int) {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	code := fn()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdout = old
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(data), code
}

// writeFile writes one file under dir.
func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestREQ_REG_02_Add covers gx add: files land at their targets, dialog
// pulls button, gx.lock records both and the base snapshots are stored
// (REQ-REG-02).
func TestREQ_REG_02_Add(t *testing.T) {
	f := registryFixture(t)
	app := appWithRegistry(t, f.out)
	if code := gxcli.Main([]string{"add", "dialog", app}); code != 0 {
		t.Fatalf("add exit = %d", code)
	}
	for _, rel := range []string{
		"ui/dialog/Dialog.gx",
		"ui/button/Button.gx",
		".gx/base/dialog@0.1.0/item.json",
		".gx/base/button@0.1.0/item.json",
	} {
		if _, err := os.Stat(filepath.Join(app, filepath.FromSlash(rel))); err != nil {
			t.Errorf("missing %s: %v", rel, err)
		}
	}
	raw, err := os.ReadFile(filepath.Join(app, "gx.lock"))
	if err != nil {
		t.Fatal(err)
	}
	var lock struct {
		Registry struct {
			Items map[string]struct {
				Version string            `json:"version"`
				Files   map[string]string `json:"files"`
			} `json:"items"`
		} `json:"registry"`
	}
	if err := json.Unmarshal(raw, &lock); err != nil {
		t.Fatalf("gx.lock: %v\n%s", err, raw)
	}
	if len(lock.Registry.Items) != 2 {
		t.Fatalf("gx.lock records %d items, want 2:\n%s", len(lock.Registry.Items), raw)
	}
	dialog, ok := lock.Registry.Items["dialog"]
	if !ok || dialog.Version != "0.1.0" {
		t.Fatalf("gx.lock dialog = %+v", dialog)
	}
	if dialog.Files["ui/dialog/Dialog.gx"] == "" {
		t.Fatalf("gx.lock has no dialog file hash: %+v", dialog.Files)
	}
	// A version that the registry does not serve fails before any write.
	other := appWithRegistry(t, registryFixture(t).out)
	if code := gxcli.Main([]string{"add", "dialog@9.9.9", other}); code == 0 {
		t.Fatal("add accepted a missing version")
	}
	if _, err := os.Stat(filepath.Join(other, "ui/dialog/Dialog.gx")); !os.IsNotExist(err) {
		t.Fatalf("failed add wrote a file (err %v)", err)
	}
}

// TestREQ_REG_02_AddKeepsLock covers a second gx add: the lock entries of an
// earlier install stay (REQ-REG-02).
func TestREQ_REG_02_AddKeepsLock(t *testing.T) {
	f := registryFixture(t)
	app := appWithRegistry(t, f.out)
	// dialog pulls button; the second add then names button alone.
	for _, item := range []string{"dialog", "button"} {
		if code := gxcli.Main([]string{"add", item, app}); code != 0 {
			t.Fatalf("add %s exit = %d", item, code)
		}
	}
	raw, err := os.ReadFile(filepath.Join(app, "gx.lock"))
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []string{"button", "dialog"} {
		if !strings.Contains(string(raw), `"`+item+`": {`) {
			t.Errorf("gx.lock lost %s after the second add:\n%s", item, raw)
		}
	}
}

// TestREQ_REG_03_DiffUpdate covers gx diff and gx update: a local edit and a
// different upstream edit merge cleanly, the lock and the base snapshot move
// to the new version, and the old snapshot goes (REQ-REG-03).
func TestREQ_REG_03_DiffUpdate(t *testing.T) {
	f := registryFixture(t)
	app := appWithRegistry(t, f.out)
	if code := gxcli.Main([]string{"add", "dialog", app}); code != 0 {
		t.Fatalf("add exit = %d", code)
	}
	writeFile(t, app, "ui/dialog/Dialog.gx", "package dialog\n\n// local note\nprops {\n  Title string\n}\n\n<dialog>{p.Title}</dialog>\n")

	diff, code := captureStdout(t, func() int { return gxcli.Main([]string{"diff", "dialog", app}) })
	if code != 0 {
		t.Fatalf("diff exit = %d\n%s", code, diff)
	}
	if !strings.Contains(diff, "local") {
		t.Fatalf("diff does not report the local change:\n%s", diff)
	}

	writeRegistryItem(t, f.src, registryItem{
		name:    "dialog",
		version: "0.2.0",
		deps:    []string{"button"},
		files:   map[string]string{"Dialog.gx": "package dialog\n\nprops {\n  Title string\n}\n\n<dialog class=\"x\">{p.Title}</dialog>\n"},
		target:  "ui/dialog/Dialog.gx",
	})
	f.build(t)

	diff, code = captureStdout(t, func() int { return gxcli.Main([]string{"diff", "dialog", app}) })
	if code != 0 {
		t.Fatalf("diff exit = %d\n%s", code, diff)
	}
	if !strings.Contains(diff, "merged") {
		t.Fatalf("diff does not report the mergeable change:\n%s", diff)
	}

	if code := gxcli.Main([]string{"update", "dialog", app}); code != 0 {
		t.Fatalf("update exit = %d", code)
	}
	merged, err := os.ReadFile(filepath.Join(app, "ui/dialog/Dialog.gx"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(merged), "// local note") || !strings.Contains(string(merged), `<dialog class="x">`) {
		t.Fatalf("update lost a change:\n%s", merged)
	}
	if _, err := os.Stat(filepath.Join(app, ".gx/base/dialog@0.2.0/item.json")); err != nil {
		t.Fatalf("no new base snapshot: %v", err)
	}
	if _, err := os.Stat(filepath.Join(app, ".gx/base/dialog@0.1.0")); !os.IsNotExist(err) {
		t.Fatalf("old base snapshot remains (err %v)", err)
	}
	raw, err := os.ReadFile(filepath.Join(app, "gx.lock"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"0.2.0"`) {
		t.Fatalf("gx.lock did not move to the new version:\n%s", raw)
	}
}

// TestREQ_REG_03_Conflicts covers gx update: two edits of the same line
// write markers and are reported, and the recorded base makes a second
// update a no-op (REQ-REG-03).
func TestREQ_REG_03_Conflicts(t *testing.T) {
	f := registryFixture(t)
	app := appWithRegistry(t, f.out)
	if code := gxcli.Main([]string{"add", "dialog", app}); code != 0 {
		t.Fatalf("add exit = %d", code)
	}
	writeFile(t, app, "ui/dialog/Dialog.gx", "package dialog\n\nprops {\n  Title string\n}\n\n<dialog>LOCAL</dialog>\n")
	writeRegistryItem(t, f.src, registryItem{
		name:    "dialog",
		version: "0.2.0",
		deps:    []string{"button"},
		files:   map[string]string{"Dialog.gx": "package dialog\n\nprops {\n  Title string\n}\n\n<dialog>UPSTREAM</dialog>\n"},
		target:  "ui/dialog/Dialog.gx",
	})
	f.build(t)

	out, code := captureStdout(t, func() int { return gxcli.Main([]string{"update", "dialog", app}) })
	if code == 0 {
		t.Fatalf("update accepted a conflict:\n%s", out)
	}
	merged, err := os.ReadFile(filepath.Join(app, "ui/dialog/Dialog.gx"))
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{"<<<<<<< local", "=======", ">>>>>>> upstream", "LOCAL", "UPSTREAM"} {
		if !strings.Contains(string(merged), marker) {
			t.Fatalf("merged file lacks %q:\n%s", marker, merged)
		}
	}
	// The recorded base is the upstream; a second update keeps the marked
	// file and does not add markers again.
	before := string(merged)
	if code := gxcli.Main([]string{"update", "dialog", app}); code != 0 {
		t.Fatalf("second update exit = %d, want 0", code)
	}
	after, err := os.ReadFile(filepath.Join(app, "ui/dialog/Dialog.gx"))
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != before {
		t.Fatalf("second update changed the resolved file:\n%s", after)
	}
}

// TestREQ_REG_04_Registries covers several registries in gx.toml with
// namespaces and auth headers, and a named official registry as the default
// of a bare item name (REQ-REG-04).
func TestREQ_REG_04_Registries(t *testing.T) {
	f := registryFixture(t)
	private := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		http.ServeFile(w, r, filepath.Join(f.out, filepath.FromSlash(strings.TrimPrefix(r.URL.Path, "/"))))
	}))
	defer private.Close()

	app := t.TempDir()
	writeFile(t, app, "gx.toml", "[registries.acme]\nurl = \""+private.URL+"\"\nheaders = \"Authorization: Bearer secret\"\n")
	if code := gxcli.Main([]string{"add", "@acme/dialog", app}); code != 0 {
		t.Fatalf("namespaced add exit = %d", code)
	}
	if _, err := os.Stat(filepath.Join(app, "ui/dialog/Dialog.gx")); err != nil {
		t.Fatalf("namespaced add wrote no file: %v", err)
	}

	// The header is required: without it the private registry refuses.
	app2 := t.TempDir()
	writeFile(t, app2, "gx.toml", "[registries.acme]\nurl = \""+private.URL+"\"\n")
	if code := gxcli.Main([]string{"add", "@acme/dialog", app2}); code == 0 {
		t.Fatal("add without the auth header succeeded")
	}
	if _, err := os.Stat(filepath.Join(app2, "ui/dialog/Dialog.gx")); !os.IsNotExist(err) {
		t.Fatalf("failed namespaced add wrote a file (err %v)", err)
	}

	// An unknown namespace is an error.
	if code := gxcli.Main([]string{"add", "@nope/button", app}); code == 0 {
		t.Fatal("add accepted an unknown namespace")
	}

	// A named official registry is the default for a bare item name.
	app3 := t.TempDir()
	writeFile(t, app3, "gx.toml", "[registries]\nofficial = \""+f.out+"\"\n")
	if code := gxcli.Main([]string{"add", "button", app3}); code != 0 {
		t.Fatalf("bare add via the official registry exit = %d", code)
	}
}

// TestREQ_REG_05_RegistryImports covers the cross-item import rewrite: an
// item that imports a sibling lands with the app module path, and an app
// without go.mod cannot install it (REQ-REG-05).
func TestREQ_REG_05_RegistryImports(t *testing.T) {
	f := &fixture{src: t.TempDir(), out: t.TempDir()}
	writeRegistryItem(t, f.src, registryItem{
		name:   "button",
		files:  map[string]string{"Button.gx": "package button\n\nprops {\n  Label string\n}\n\n<button>{p.Label}</button>\n"},
		target: "ui/button/Button.gx",
	})
	writeRegistryItem(t, f.src, registryItem{
		name:   "dialog",
		deps:   []string{"button"},
		files:  map[string]string{"Dialog.gx": "package dialog\n\nimport \"github.com/alternayte/gx/registry/button\"\n\nprops {\n  Label string\n}\n\n<button.Button label={p.Label} />\n"},
		target: "ui/dialog/Dialog.gx",
	})
	f.build(t)

	app := appWithRegistry(t, f.out)
	writeFile(t, app, "go.mod", "module app\n\ngo 1.25.0\n")
	if code := gxcli.Main([]string{"add", "dialog", app}); code != 0 {
		t.Fatalf("add exit = %d", code)
	}
	for _, rel := range []string{"ui/dialog/Dialog.gx", ".gx/base/dialog@0.1.0/item.json"} {
		data, err := os.ReadFile(filepath.Join(app, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), `app/ui/button`) {
			t.Fatalf("%s was not rewritten:\n%s", rel, data)
		}
	}

	// Without go.mod the same item cannot resolve its sibling import.
	bare := appWithRegistry(t, f.out)
	if code := gxcli.Main([]string{"add", "dialog", bare}); code == 0 {
		t.Fatal("add accepted a cross-item import without go.mod")
	}
}

// TestREQ_REG_06_LintCommand covers the gx registry lint command: a valid
// item passes, and an item without fixtures or usage sections fails
// (REQ-REG-06).
func TestREQ_REG_06_LintCommand(t *testing.T) {
	src := t.TempDir()
	dir := filepath.Join(src, "button")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{"name":"button","version":"0.1.0","description":"A button.","kind":"component","files":[{"path":"Button.gx","target":"ui/button/Button.gx"},{"path":"Button.fixtures.go","target":"ui/button/Button.fixtures.go"}]}`
	usage := "# Button\n\n## Usage\n\n```gx\n<button.Button />\n```\n\n## Do\n\n- Use one primary button.\n\n## Don't\n\n- Do not use a button as a link.\n\n## Keyboard\n\n| Key | Action |\n| --- | --- |\n| Enter | Activates the button. |\n"
	writeFile(t, src, "button/gx-item.json", manifest)
	writeFile(t, src, "button/Button.gx", "package button\n\n<button></button>\n")
	writeFile(t, src, "button/Button.fixtures.go", "package button\n\nimport \"github.com/alternayte/gx\"\n\nvar ButtonFixtures = gx.Fixtures[ButtonProps]{}\n")
	writeFile(t, src, "button/USAGE.md", usage)

	if code := gxcli.Main([]string{"registry", "lint", src}); code != 0 {
		t.Fatalf("lint exit = %d", code)
	}
	if err := os.Remove(filepath.Join(dir, "Button.fixtures.go")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, src, "button/USAGE.md", "# Button\n\n## Usage\n\n```gx\n<button.Button />\n```\n")
	if code := gxcli.Main([]string{"registry", "lint", src}); code == 0 {
		t.Fatal("lint accepted an item without fixtures and sections")
	}
}

// TestREQ_REG_12_RoundTrip covers gx registry build: build a publishable
// registry from an item folder, serve it over HTTP and add an item from it
// (REQ-REG-12).
func TestREQ_REG_12_RoundTrip(t *testing.T) {
	f := registryFixture(t)
	served := t.TempDir()
	if code := gxcli.Main([]string{"registry", "build", "--out", served, f.src}); code != 0 {
		t.Fatalf("registry build exit = %d", code)
	}
	for _, rel := range []string{"index.json", "items/button.json", "items/dialog.json", "schema/item.schema.json"} {
		if _, err := os.Stat(filepath.Join(served, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("build output missing %s: %v", rel, err)
		}
	}
	srv := httptest.NewServer(http.FileServer(http.Dir(served)))
	defer srv.Close()
	app := t.TempDir()
	writeFile(t, app, "gx.toml", "[registry]\nurl = \""+srv.URL+"\"\n")
	if code := gxcli.Main([]string{"add", "dialog", app}); code != 0 {
		t.Fatalf("add from the served registry exit = %d", code)
	}
	if _, err := os.Stat(filepath.Join(app, "ui/button/Button.gx")); err != nil {
		t.Fatalf("round trip did not pull the dependency: %v", err)
	}
}

// TestSI_09_TamperedRegistry covers gx add: a file whose bytes do not match
// its published hash stops the install before any write, and the installer
// never runs registry code (SI-09).
func TestSI_09_TamperedRegistry(t *testing.T) {
	f := registryFixture(t)
	path := filepath.Join(f.out, "items", "dialog.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var item map[string]any
	if err := json.Unmarshal(raw, &item); err != nil {
		t.Fatal(err)
	}
	files, ok := item["files"].([]any)
	if !ok || len(files) == 0 {
		t.Fatalf("published item has no files:\n%s", raw)
	}
	file := files[0].(map[string]any)
	file["content"] = "package dialog\n\n// tampered\n"
	tampered, err := json.MarshalIndent(item, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, tampered, 0o644); err != nil {
		t.Fatal(err)
	}

	app := appWithRegistry(t, f.out)
	if code := gxcli.Main([]string{"add", "dialog", app}); code == 0 {
		t.Fatal("add accepted a tampered item")
	}
	for _, rel := range []string{"ui/dialog/Dialog.gx", "ui/button/Button.gx", ".gx/base"} {
		if _, err := os.Stat(filepath.Join(app, filepath.FromSlash(rel))); !os.IsNotExist(err) {
			t.Fatalf("tampered add wrote %s (err %v)", rel, err)
		}
	}

	// The installer reads JSON and writes files; it never runs registry
	// code and never loads a plugin.
	out, err := exec.Command("go", "list", "-deps", "github.com/alternayte/gx/internal/registry").Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	deps := string(out)
	for _, bad := range []string{"os/exec", "plugin"} {
		for _, line := range strings.Split(deps, "\n") {
			if line == bad {
				t.Fatalf("the installer depends on %s", bad)
			}
		}
	}
}
