package registry_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/registry"
)

// TestREQ_REG_06_Lint covers the registry lint: an item without a fixtures
// file, usage sections or a keyboard spec fails, and the official registry
// passes (REQ-REG-06).
func TestREQ_REG_06_Lint(t *testing.T) {
	root := repoRoot(t)
	findings, err := registry.Lint(filepath.Join(root, "registry"))
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
	if err := os.WriteFile(filepath.Join(dir, "Button.gx"), []byte("package button\n\n<button></button>\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "USAGE.md"), []byte("# Button\n\n## Usage\n\n```gx\n<button.Button />\n```\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	findings, err = registry.Lint(bad)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(findings, "\n")
	for _, want := range []string{"fixtures.go", "Do section", "Don't section", "Keyboard section"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("lint does not report %q:\n%s", want, joined)
		}
	}
}
