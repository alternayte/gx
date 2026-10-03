package editors

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

// TestREQ_TLS_05_VSCodeManifest covers the VS Code extension: the gx
// language, the TextMate grammar with embedded languages, the format-on-save
// default and the LSP launcher (REQ-TLS-05).
func TestREQ_TLS_05_VSCodeManifest(t *testing.T) {
	dir := filepath.Join(repoRoot(t), "editors", "vscode")
	var manifest struct {
		Main       string `json:"main"`
		Engines    struct{ VSCode string `json:"vscode"` } `json:"engines"`
		Contributes struct {
			Languages []struct {
				ID         string   `json:"id"`
				Extensions []string `json:"extensions"`
				Config     string   `json:"configuration"`
			} `json:"languages"`
			Grammars []struct {
				Language string `json:"language"`
				Scope    string `json:"scopeName"`
				Path     string `json:"path"`
			} `json:"grammars"`
			Commands []struct {
				Command string `json:"command"`
			} `json:"commands"`
			ConfigurationDefaults map[string]map[string]any `json:"configurationDefaults"`
		} `json:"contributes"`
	}
	readJSON(t, filepath.Join(dir, "package.json"), &manifest)
	if len(manifest.Contributes.Languages) != 1 {
		t.Fatalf("languages = %+v", manifest.Contributes.Languages)
	}
	lang := manifest.Contributes.Languages[0]
	if lang.ID != "gx" || len(lang.Extensions) != 1 || lang.Extensions[0] != ".gx" {
		t.Fatalf("language = %+v", lang)
	}
	if _, err := os.Stat(filepath.Join(dir, lang.Config)); err != nil {
		t.Fatalf("language configuration: %v", err)
	}
	if len(manifest.Contributes.Grammars) != 1 || manifest.Contributes.Grammars[0].Scope != "source.gx" {
		t.Fatalf("grammars = %+v", manifest.Contributes.Grammars)
	}
	grammarPath := filepath.Join(dir, manifest.Contributes.Grammars[0].Path)
	grammar := readBody(t, grammarPath)
	for _, embedded := range []string{"source.go", "source.css", "source.js", "source.ts"} {
		if !strings.Contains(grammar, embedded) {
			t.Fatalf("grammar lacks the embedded language %q", embedded)
		}
	}
	var parsed map[string]any
	readJSON(t, grammarPath, &parsed)
	if parsed["scopeName"] != "source.gx" {
		t.Fatalf("grammar scope = %v", parsed["scopeName"])
	}

	defaults := manifest.Contributes.ConfigurationDefaults["[gx]"]
	if defaults["editor.formatOnSave"] != true {
		t.Fatalf("format on save default = %+v", defaults)
	}
	if defaults["editor.defaultFormatter"] != "alternayte.gx-vscode" {
		t.Fatalf("default formatter = %v", defaults["editor.defaultFormatter"])
	}
	foundDev := false
	for _, c := range manifest.Contributes.Commands {
		if c.Command == "gx.dev" {
			foundDev = true
		}
	}
	if !foundDev {
		t.Fatalf("commands = %+v", manifest.Contributes.Commands)
	}
	ext := readBody(t, filepath.Join(dir, manifest.Main))
	if !strings.Contains(ext, `"lsp"`) {
		t.Fatalf("the extension does not start gx lsp")
	}
	if !strings.Contains(ext, "registerTaskProvider") {
		t.Fatalf("the extension has no gx dev task provider")
	}
}

func readJSON(t *testing.T, path string, v any) {
	t.Helper()
	if err := json.Unmarshal([]byte(readBody(t, path)), v); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

func readBody(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
