package registry_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/registry"
)

// repoRoot returns the root of the gx repository.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

// readIndex reads the index.json of a published registry.
func readIndex(t *testing.T, out string) (registry.Index, error) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(out, "index.json"))
	if err != nil {
		return registry.Index{}, err
	}
	var index registry.Index
	if err := json.Unmarshal(data, &index); err != nil {
		return registry.Index{}, err
	}
	return index, nil
}

// TestREQ_REG_05_OfficialRegistry covers the official registry: every
// tier 1 and tier 2 component is an item, the five blocks exist, the
// published registry validates and the gallery shows every fixture
// (REQ-REG-05).
func TestREQ_REG_05_OfficialRegistry(t *testing.T) {
	root := repoRoot(t)
	src := filepath.Join(root, "registry")
	out := t.TempDir()
	if _, err := registry.Build(src, out); err != nil {
		t.Fatalf("registry build: %v", err)
	}
	if err := registry.ValidateRegistry(out); err != nil {
		t.Fatalf("published registry: %v", err)
	}
	index, err := readIndex(t, out)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, item := range index.Items {
		got[item.Name] = item.Kind
	}
	tiers := []string{
		// Tier 1.
		"button", "button-group", "badge", "card", "alert", "avatar", "separator",
		"skeleton", "table", "breadcrumb", "pagination", "label", "input",
		"input-group", "textarea", "checkbox", "radio-group", "switch", "progress",
		"kbd", "spinner", "empty", "field", "item", "aspect-ratio",
		// Tier 2.
		"dialog", "alert-dialog", "sheet", "drawer", "popover", "dropdown-menu",
		"context-menu", "menubar", "tooltip", "hover-card", "accordion",
		"collapsible", "tabs", "toggle", "toggle-group", "navigation-menu",
		"sidebar", "select", "scroll-area", "slider", "toast",
		// Tier 3: islands (REQ-REG-14).
		"calendar", "date-picker", "command", "combobox", "carousel", "chart",
		"resizable", "input-otp",
	}
	for _, name := range tiers {
		if kind, ok := got[name]; !ok || kind != "component" {
			t.Errorf("tier item %s = %q, want a component item", name, kind)
		}
	}
	for _, name := range []string{"login", "signup", "app-shell", "settings-form", "data-table-page"} {
		if kind, ok := got[name]; !ok || kind != "block" {
			t.Errorf("block %s = %q, want a block item", name, kind)
		}
	}

	// The gallery has one entry per component and no missing fixture.
	gallery := filepath.Join(src, "gxdev_gallery", "gallery_gx.go")
	data, err := os.ReadFile(gallery)
	if err != nil {
		t.Fatalf("read the gallery: %v", err)
	}
	if strings.Contains(string(data), "Missing: true") {
		t.Errorf("the gallery reports a missing fixture:\n%s", data)
	}
	for _, name := range append(tiers, "login", "signup", "app-shell", "settings-form", "data-table-page") {
		components := componentsOf(t, filepath.Join(src, name))
		found := false
		for _, component := range components {
			if strings.Contains(string(data), `Component: "`+component+`"`) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("the gallery lacks an item of %s (%v)", name, components)
		}
	}
}

// componentsOf lists the component names of one item folder.
func componentsOf(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".gx") {
			out = append(out, strings.TrimSuffix(entry.Name(), ".gx"))
		}
	}
	return out
}

// TestREQ_REG_14_TierThreeItems checks the eight tier 3 items: each is a
// component item with an island file, fixtures for its components and no
// npm dependency. TestREQ_REG_06_Lint checks the usage page and the keyboard
// table of every item.
func TestREQ_REG_14_TierThreeItems(t *testing.T) {
	root := filepath.Join("..", "..", "registry")
	for _, name := range []string{"calendar", "date-picker", "command", "combobox", "carousel", "chart", "resizable", "input-otp"} {
		data, err := os.ReadFile(filepath.Join(root, name, "gx-item.json"))
		if err != nil {
			t.Fatal(err)
		}
		var manifest struct {
			Kind                 string
			Files                []struct{ Path string }
			JSPins               []string
			RegistryDependencies []string
		}
		if err := json.Unmarshal(data, &manifest); err != nil {
			t.Fatal(err)
		}
		if manifest.Kind != "component" || len(manifest.JSPins) != 0 {
			t.Errorf("%s: kind %q, js pins %v; want a component with no npm package", name, manifest.Kind, manifest.JSPins)
		}
		islands, fixtures := 0, 0
		for _, f := range manifest.Files {
			switch {
			case strings.HasSuffix(f.Path, ".ts"):
				islands++
				src, err := os.ReadFile(filepath.Join(root, name, f.Path))
				if err != nil || !strings.Contains(string(src), "export default mount") {
					t.Errorf("%s: %s is not an island file: %v", name, f.Path, err)
				}
			case strings.HasSuffix(f.Path, ".fixtures.go"):
				fixtures++
			}
		}
		// The date picker has no island of its own: it is a popover with a
		// calendar.
		wantIslands := 1
		if name == "date-picker" {
			wantIslands = 0
			if strings.Join(manifest.RegistryDependencies, " ") != "calendar popover" {
				t.Errorf("date-picker depends on %v", manifest.RegistryDependencies)
			}
		}
		if islands != wantIslands || fixtures == 0 {
			t.Errorf("%s: %d island files (want %d) and %d fixture files", name, islands, wantIslands, fixtures)
		}
	}
}
