package wcpin_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
	"github.com/alternayte/gx/internal/islands"
	"github.com/alternayte/gx/internal/jspin"
	"github.com/alternayte/gx/internal/wcpin"
)

// A custom elements manifest in the form of a component library: the
// manifest is in dist/, and each module path names the file that defines
// one element.
const libManifest = `{
  "schemaVersion": "1.0.0",
  "modules": [
    {
      "kind": "javascript-module",
      "path": "components/button/button.js",
      "declarations": [
        {
          "kind": "class", "name": "UiButton", "tagName": "ui-button", "customElement": true,
          "summary": "Buttons represent actions.\nMore text.",
          "attributes": [
            {"name": "variant", "type": {"text": "'neutral' | 'brand' | 'danger'"}, "description": "The theme variant.", "fieldName": "variant"},
            {"name": "size", "type": {"text": "\"s\" | \"m\" | undefined"}},
            {"name": "disabled", "type": {"text": "boolean"}},
            {"name": "tabindex", "type": {"text": "number | undefined"}},
            {"name": "href", "type": {"text": "string"}},
            {"name": "target", "type": {"text": "'_blank' | '_self' | string"}},
            {"name": "label"},
            {"name": "label"}
          ],
          "members": [{"kind": "field", "name": "variant", "attribute": "variant", "reflects": true}],
          "events": [{"name": "ui-invalid"}, {"name": "focus"}],
          "slots": [{"name": ""}, {"name": "start"}, {"name": "end"}]
        },
        {"kind": "class", "name": "Helper"}
      ]
    },
    {
      "kind": "javascript-module",
      "path": "components/radio-group/radio-group.js",
      "declarations": [
        {"kind": "class", "name": "UiRadioGroup", "tagName": "ui-radio-group", "customElement": true, "attributes": [], "events": [], "slots": []}
      ]
    }
  ]
}`

// A manifest of source files: the module path is a .ts file, so the entry
// of the package defines the element.
const sourceManifest = `{
  "schemaVersion": "1.0.0",
  "modules": [{
    "kind": "javascript-module",
    "path": "src/relative-time-element.ts",
    "declarations": [{"kind": "class", "name": "RelativeTimeElement", "tagName": "relative-time", "customElement": true,
      "attributes": [{"name": "datetime"}, {"name": "tense"}], "events": [{"name": "relative-time-updated"}]}]
  }]
}`

func cdn(t *testing.T) *httptest.Server {
	t.Helper()
	files := map[string]string{
		"/npm/@acme/ui@2.1.0/package.json":                                    `{"name": "@acme/ui", "customElements": "dist/custom-elements.json"}`,
		"/npm/@acme/ui@2.1.0/dist/custom-elements.json":                       libManifest,
		"/npm/@acme/ui@2.1.0/dist/components/button/button.js/+esm":           "import{base}from\"/npm/@acme/ui@2.1.0/dist/base.js/+esm\";customElements.define(\"ui-button\",class extends base{connectedCallback(){this.attachShadow({mode:\"open\"}).innerHTML=\"<slot name=start></slot><slot></slot>\"}});\n",
		"/npm/@acme/ui@2.1.0/dist/components/radio-group/radio-group.js/+esm": "import{base}from\"/npm/@acme/ui@2.1.0/dist/base.js/+esm\";customElements.define(\"ui-radio-group\",class extends base{});\n",
		"/npm/@acme/ui@2.1.0/dist/base.js/+esm":                               "export const base=class extends HTMLElement{static marker=\"SHARED-ELEMENT-BASE\"};\n",
		"/npm/relative-time-element@5.0.0/package.json":                       `{"name": "relative-time-element"}`,
		"/npm/relative-time-element@5.0.0/custom-elements.json":               sourceManifest,
		"/npm/relative-time-element@5.0.0/+esm":                               "customElements.define(\"relative-time\",class extends HTMLElement{});\n",
		"/npm/plain@1.0.0/package.json":                                       `{"name": "plain"}`,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func elementSet(t *testing.T, dir, pkg string) compiler.ElementSet {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(pkg), compiler.ElementsFile))
	if err != nil {
		t.Fatal(err)
	}
	var set compiler.ElementSet
	if err := json.Unmarshal(data, &set); err != nil {
		t.Fatal(err)
	}
	return set
}

func TestREQ_ISL_09_PinReadsTheManifest(t *testing.T) {
	srv := cdn(t)
	dir := t.TempDir()
	res, err := wcpin.Pin(context.Background(), wcpin.Options{Dir: dir, Spec: "@acme/ui@2.1.0", BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	// The tags share the prefix "ui", which is the package name.
	if res.Package != "ui/ui" || strings.Join(res.Tags, " ") != "ui-button ui-radio-group" {
		t.Fatalf("result = %+v", res)
	}
	set := elementSet(t, dir, "ui/ui")
	if set.Package != "@acme/ui" || set.Version != "2.1.0" || len(set.Elements) != 2 {
		t.Fatalf("set = %+v", set)
	}
	button := set.Elements[0]
	if button.Name != "Button" || button.Tag != "ui-button" || button.Doc != "Buttons represent actions." ||
		button.Module != "@acme/ui/dist/components/button/button.js" {
		t.Fatalf("button = %+v", button)
	}
	kinds := map[string]string{}
	for _, a := range button.Attributes {
		kinds[a.Name] = a.Kind + " " + strings.Join(a.Values, ",")
	}
	want := map[string]string{
		"variant":  "enum neutral,brand,danger",
		"size":     "enum s,m",
		"disabled": "bool ",
		"tabindex": "number ",
		"href":     "string ",
		// A union with string in it takes every string.
		"target": "string ",
		"label":  "string ",
	}
	if len(kinds) != len(want) {
		t.Fatalf("attributes = %v", kinds)
	}
	for name, kind := range want {
		if kinds[name] != kind {
			t.Errorf("attribute %s = %q, want %q", name, kinds[name], kind)
		}
	}
	if strings.Join(button.Events, " ") != "ui-invalid focus" || strings.Join(button.Slots, " ") != "start end" {
		t.Fatalf("events = %v, slots = %v", button.Events, button.Slots)
	}
	if set.Elements[1].Name != "RadioGroup" {
		t.Fatalf("second element = %+v", set.Elements[1])
	}
	if src, err := os.ReadFile(filepath.Join(dir, "ui", "ui", "elements_gx.go")); err != nil || !strings.Contains(string(src), "\npackage ui\n") {
		t.Fatalf("elements_gx.go = %s, %v", src, err)
	}

	// The module of each element is a pin of gx.lock, with the file they
	// share.
	lock, err := jspin.LoadLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	if lock.Pins["@acme/ui/dist/components/button/button.js"].File != "js/vendor/@acme/ui@2.1.0/dist/components/button/button.js.js" || len(lock.Pins) != 2 || len(lock.Files) != 3 {
		t.Fatalf("lock = %+v", lock)
	}
	if err := jspin.Verify(dir, lock); err != nil {
		t.Fatal(err)
	}
}

func TestREQ_ISL_09_PinOptions(t *testing.T) {
	srv := cdn(t)
	dir := t.TempDir()
	res, err := wcpin.Pin(context.Background(), wcpin.Options{Dir: dir, Spec: "@acme/ui@2.1.0", BaseURL: srv.URL, As: "acme", Out: "web", Elements: []string{"ui-button"}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Package != "web/acme" || strings.Join(res.Tags, " ") != "ui-button" {
		t.Fatalf("result = %+v", res)
	}
	// One element has no shared prefix to remove.
	if set := elementSet(t, dir, "web/acme"); set.Elements[0].Name != "UiButton" {
		t.Fatalf("name = %s", set.Elements[0].Name)
	}
	if lock, _ := jspin.LoadLock(dir); len(lock.Pins) != 1 {
		t.Fatalf("pins = %v, want the button module only", lock.Pins)
	}

	// A manifest of source files: the entry of the package is the module,
	// and the Go package name comes from the npm name.
	res, err = wcpin.Pin(context.Background(), wcpin.Options{Dir: dir, Spec: "relative-time-element@5.0.0", BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	if res.Package != "ui/relativetimeelement" {
		t.Fatalf("package = %s", res.Package)
	}
	set := elementSet(t, dir, "ui/relativetimeelement")
	if el := set.Elements[0]; el.Name != "RelativeTime" || el.Module != "relative-time-element" || el.Attributes[0].Kind != "string" {
		t.Fatalf("element = %+v", el)
	}
}

func TestREQ_ISL_09_PinErrors(t *testing.T) {
	srv := cdn(t)
	for spec, want := range map[string]string{
		"@acme/ui":      "is not <pkg>@<version>",
		"plain@1.0.0":   "has no custom elements manifest",
		"missing@1.0.0": "404",
	} {
		dir := t.TempDir()
		_, err := wcpin.Pin(context.Background(), wcpin.Options{Dir: dir, Spec: spec, BaseURL: srv.URL})
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Pin(%s) = %v, want %q", spec, err, want)
		}
		if entries, _ := os.ReadDir(dir); len(entries) != 0 {
			t.Errorf("Pin(%s) wrote %v", spec, entries)
		}
	}
	dir := t.TempDir()
	_, err := wcpin.Pin(context.Background(), wcpin.Options{Dir: dir, Spec: "@acme/ui@2.1.0", BaseURL: srv.URL, Elements: []string{"ui-button", "ui-card"}})
	if err == nil || !strings.Contains(err.Error(), "has no element ui-card") {
		t.Fatalf("with an unknown element: %v", err)
	}
	_, err = wcpin.Pin(context.Background(), wcpin.Options{Dir: dir, Spec: "@acme/ui@2.1.0", BaseURL: srv.URL, As: "My-UI"})
	if err == nil || !strings.Contains(err.Error(), "is not a Go package name") {
		t.Fatalf("with a bad package name: %v", err)
	}
}

// From the pin to the bundle: a page uses one imported element, the check
// passes, and the bundle has one entry for the module of that element.
func TestREQ_ISL_09_PinnedElementIsBundled(t *testing.T) {
	srv := cdn(t)
	_, file, _, _ := runtime.Caller(0)
	repo := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	dir := t.TempDir()
	for rel, content := range map[string]string{
		"go.mod":       "module app\n\ngo 1.25.0\n\nrequire github.com/alternayte/gx v0.0.0\n\nreplace github.com/alternayte/gx => " + filepath.ToSlash(repo) + "\n",
		"home/Page.gx": "package home\n\nimport \"app/ui/ui\"\n\n<main><ui.Button variant=\"brand\"><:start>go</:start>Save</ui.Button></main>\n",
	} {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := wcpin.Pin(context.Background(), wcpin.Options{Dir: dir, Spec: "@acme/ui@2.1.0", BaseURL: srv.URL}); err != nil {
		t.Fatal(err)
	}
	srv.Close()
	if diags := compiler.Check(dir); len(diags) != 0 {
		t.Fatalf("check: %v", diags)
	}
	b, err := islands.Build(dir, islands.Options{Minify: true})
	if err != nil {
		t.Fatal(err)
	}
	entry := b.Entries["@acme/ui/dist/components/button/button.js"]
	if len(b.Entries) != 1 || !strings.HasPrefix(entry, "elements/acme-ui-dist-components-button-button-") {
		t.Fatalf("entries = %v", b.Entries)
	}
	body := string(b.Files[entry])
	if !strings.Contains(body, `"ui-button"`) || !strings.Contains(body, "SHARED-ELEMENT-BASE") {
		t.Fatalf("the entry does not define the element:\n%s", body)
	}
	if strings.Contains(body, "ui-radio-group") {
		t.Fatalf("the entry holds an element that no page uses:\n%s", body)
	}
}
