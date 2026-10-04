package registry_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/registry"
)

// writeItem writes one source item folder and returns its root.
func writeItem(t *testing.T, dir, name string, extra map[string]string) {
	t.Helper()
	files := map[string]string{
		"Aside.gx": "package " + name + "\n",
		"aside.go": "package " + name + "\n",
		"USAGE.md": "# " + name + "\n",
		"gx-item.json": `{
  "name": "` + name + `",
  "version": "0.1.0",
  "description": "A test item.",
  "kind": "component",
  "files": [
    {"path": "Aside.gx", "target": "ui/` + name + `/Aside.gx"},
    {"path": "aside.go", "target": "ui/` + name + `/aside.go"}
  ],
  "requiredTokens": ["--background"]
}
`,
	}
	for k, v := range extra {
		files[k] = v
	}
	root := filepath.Join(dir, name)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	for rel, body := range files {
		if err := os.WriteFile(filepath.Join(root, rel), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestREQ_REG_01_Format covers the registry item format: build, hashes,
// index and validation (REQ-REG-01).
func TestREQ_REG_01_Format(t *testing.T) {
	src := t.TempDir()
	writeItem(t, src, "card", nil)
	out := t.TempDir()
	index, err := registry.Build(src, out)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(index.Items) != 1 || index.Items[0].Name != "card" {
		t.Fatalf("index = %+v", index)
	}
	item, err := registry.Find(out, "card")
	if err != nil {
		t.Fatal(err)
	}
	if len(item.Files) != 2 || item.Files[0].Target != "ui/card/Aside.gx" {
		t.Fatalf("files = %+v", item.Files)
	}
	if len(item.Files[0].SHA256) != 64 {
		t.Fatalf("sha256 = %q", item.Files[0].SHA256)
	}
	if item.Usage == "" {
		t.Fatal("usage is empty")
	}
	if err := registry.ValidateRegistry(out); err != nil {
		t.Fatalf("ValidateRegistry: %v", err)
	}

	// A tampered content fails its hash.
	bad := item
	bad.Files = append([]registry.File(nil), item.Files...)
	bad.Files[0].Content += "// tampered\n"
	if err := registry.ValidateItem(bad); err == nil {
		t.Fatal("tampered content was accepted")
	}
	// A bad kind, version and a missing usage file fail.
	for _, mutate := range []func(*registry.Item){
		func(i *registry.Item) { i.Kind = "widget" },
		func(i *registry.Item) { i.Version = "one" },
		func(i *registry.Item) { i.Usage = "" },
		func(i *registry.Item) { i.Files[0].Target = "/etc/passwd" },
		func(i *registry.Item) { i.RequiredTokens = []string{"background"} },
	} {
		bad := item
		mutate(&bad)
		if err := registry.ValidateItem(bad); err == nil {
			t.Fatalf("invalid item was accepted: %+v", bad)
		}
	}

	// Missing USAGE.md in the source fails the build.
	src2 := t.TempDir()
	writeItem(t, src2, "card", nil)
	if err := os.Remove(filepath.Join(src2, "card", "USAGE.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Build(src2, t.TempDir()); err == nil {
		t.Fatal("an item without USAGE.md was built")
	}
}

// TestREQ_REG_01_OfficialRegistry covers the checked-in registry and its
// JSON Schema (REQ-REG-01).
func TestREQ_REG_01_OfficialRegistry(t *testing.T) {
	dir := filepath.Join("..", "..", "registry")
	if err := registry.ValidateRegistry(dir); err != nil {
		t.Fatalf("the official registry does not validate: %v", err)
	}
	index, err := os.ReadFile(filepath.Join(dir, "index.json"))
	if err != nil {
		t.Fatal(err)
	}
	var parsed registry.Index
	if err := json.Unmarshal(index, &parsed); err != nil {
		t.Fatal(err)
	}
	if len(parsed.Items) < 2 {
		t.Fatalf("the official registry holds %d items", len(parsed.Items))
	}
	schema, err := registry.Schema()
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(schema, &doc); err != nil {
		t.Fatalf("schema is not JSON: %v", err)
	}
	required, _ := doc["required"].([]any)
	want := map[string]bool{"name": false, "version": false, "description": false, "kind": false, "files": false, "usage": false}
	for _, key := range required {
		if _, ok := want[key.(string)]; ok {
			want[key.(string)] = true
		}
	}
	for key, seen := range want {
		if !seen {
			t.Errorf("schema does not require %q", key)
		}
	}
	built, err := os.ReadFile(filepath.Join(dir, "schema", "item.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(built)) != strings.TrimSpace(string(schema)) {
		t.Error("the published schema differs from the embedded schema")
	}
}
