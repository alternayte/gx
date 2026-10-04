package gxcli_test

import (
	"encoding/json"
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
	name   string
	deps   []string
	files  map[string]string
	target string
}

// writeRegistryItem writes one item folder into a registry source.
func writeRegistryItem(t *testing.T, src string, item registryItem) {
	t.Helper()
	dir := filepath.Join(src, item.name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	var files []map[string]string
	for file := range item.files {
		files = append(files, map[string]string{"path": file, "target": item.target})
	}
	manifest := map[string]any{
		"name": item.name, "version": "0.1.0", "description": item.name + " test item",
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

// registryFixture builds a published registry with a button item and a
// dialog item that depends on it (REQ-REG-02).
func registryFixture(t *testing.T) string {
	t.Helper()
	src := t.TempDir()
	writeRegistryItem(t, src, registryItem{
		name:   "button",
		files:  map[string]string{"Button.gx": "package button\n\nprops {\n  Label string\n}\n\n<button>{p.Label}</button>\n"},
		target: "ui/button/Button.gx",
	})
	writeRegistryItem(t, src, registryItem{
		name:   "dialog",
		deps:   []string{"button"},
		files:  map[string]string{"Dialog.gx": "package dialog\n\nprops {\n  Title string\n}\n\n<dialog>{p.Title}</dialog>\n"},
		target: "ui/dialog/Dialog.gx",
	})
	out := t.TempDir()
	if _, err := registry.Build(src, out); err != nil {
		t.Fatal(err)
	}
	return out
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

// TestREQ_REG_02_Add covers gx add: files land at their targets, dialog
// pulls button, gx.lock records both and the base snapshots are stored
// (REQ-REG-02).
func TestREQ_REG_02_Add(t *testing.T) {
	app := appWithRegistry(t, registryFixture(t))
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
	other := appWithRegistry(t, registryFixture(t))
	if code := gxcli.Main([]string{"add", "dialog@9.9.9", other}); code == 0 {
		t.Fatal("add accepted a missing version")
	}
	if _, err := os.Stat(filepath.Join(other, "ui/dialog/Dialog.gx")); !os.IsNotExist(err) {
		t.Fatalf("failed add wrote a file (err %v)", err)
	}
}

// TestSI_09_TamperedRegistry covers gx add: a file whose bytes do not match
// its published hash stops the install before any write, and the installer
// never runs registry code (SI-09).
func TestSI_09_TamperedRegistry(t *testing.T) {
	published := registryFixture(t)
	path := filepath.Join(published, "items", "dialog.json")
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

	app := appWithRegistry(t, published)
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
