package registry_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
	"github.com/alternayte/gx/internal/registry"
)

// propDocs reads the props of one .gx source with the real parser, as the
// CLI does.
func propDocs(path string, src []byte) ([]registry.PropDoc, error) {
	file, diags := compiler.ParseFile(path, src)
	if len(diags) > 0 {
		return nil, errors.New(diags[0].Msg)
	}
	var out []registry.PropDoc
	for _, prop := range file.Props {
		out = append(out, registry.PropDoc{Name: prop.Name, Doc: prop.Doc})
	}
	return out, nil
}

// TestREQ_REG_06_Lint covers the registry lint: an item without a fixtures
// file, usage sections, a keyboard spec or a prop description fails, and the
// official registry passes (REQ-REG-06).
func TestREQ_REG_06_Lint(t *testing.T) {
	root := repoRoot(t)
	findings, err := registry.Lint(filepath.Join(root, "registry"), propDocs)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) > 0 {
		t.Fatalf("the official registry has findings:\n%s", strings.Join(findings, "\n"))
	}

	bad := t.TempDir()
	dir := filepath.Join(bad, "button")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := map[string]any{
		"name": "button", "version": "0.1.0", "description": "A button.",
		"kind":  "component",
		"files": []map[string]string{{"path": "Button.gx", "target": "ui/button/Button.gx"}},
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "gx-item.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Button.gx"), []byte("package button\n\nprops {\n  // Class adds classes.\n  Class string = \"\"\n  Size  string = \"md\"\n}\n\n<button></button>\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "USAGE.md"), []byte("# Button\n\n## Usage\n\n```gx\n<button.Button />\n```\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	findings, err = registry.Lint(bad, propDocs)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(findings, "\n")
	for _, want := range []string{"fixtures.go", "Do section", "Don't section", "Keyboard section", "prop Size has no description"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("lint does not report %q:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "prop Class") {
		t.Fatalf("lint reports a documented prop:\n%s", joined)
	}
}
