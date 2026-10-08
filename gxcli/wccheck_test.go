package gxcli_test

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/alternayte/gx/gxcli"
)

// widgetsTable writes the [widgets] table of the app with one version.
func widgetsTable(t *testing.T, dir, version string) {
	t.Helper()
	toml := "[widgets]\nname = \"@acme/widgets\"\nversion = \"" + version + "\"\nserver = \"https://api.acme.dev\"\naccess = \"public\"\n"
	if err := os.WriteFile(filepath.Join(dir, "gx.toml"), []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
}

// edit replaces one text of a file of the app and generates the app again.
func edit(t *testing.T, dir, file, old, text string) {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(file))
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), old) {
		t.Fatalf("%s has no %q", file, old)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(string(src), old, text, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := gxcli.Main([]string{"generate", dir}); code != 0 {
		t.Fatalf("gx generate = %d", code)
	}
}

// wcCheck runs gx wc check and returns its exit code and its error text.
func wcCheck(t *testing.T, args ...string) (int, string) {
	t.Helper()
	code := 0
	text := captureStderr(t, func() { code = gxcli.Main(append([]string{"wc", "check"}, args...)) })
	return code, text
}

// TestREQ_ISL_13_WCCheckCommand checks `gx wc check` on an app: it passes
// with no baseline, records one with --update, and then fails for an
// addition with no minor bump and for a breaking change with no major bump.
func TestREQ_ISL_13_WCCheckCommand(t *testing.T) {
	dir := widgetModule(t)
	widgetsTable(t, dir, "1.0.0")
	if code := gxcli.Main([]string{"generate", dir}); code != 0 {
		t.Fatalf("gx generate = %d", code)
	}
	baseline := filepath.Join(dir, ".gx", "base", "widgets.json")
	if code, text := wcCheck(t, dir); code != 0 {
		t.Fatalf("gx wc check with no baseline = %d: %s", code, text)
	}
	if _, err := os.Stat(baseline); err == nil {
		t.Fatal("gx wc check wrote a baseline with no --update")
	}
	if code, text := wcCheck(t, "--update", dir); code != 0 {
		t.Fatalf("gx wc check --update = %d: %s", code, text)
	}
	var recorded struct {
		Name, Version string
		Manifest      struct {
			Modules []struct {
				Declarations []struct {
					TagName string
					Events  []struct {
						Name   string
						Detail []struct {
							Name string
							Type struct{ Text string }
						}
					}
				}
			}
		}
	}
	data, err := os.ReadFile(baseline)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &recorded); err != nil {
		t.Fatal(err)
	}
	if recorded.Name != "@acme/widgets" || recorded.Version != "1.0.0" || len(recorded.Manifest.Modules) != 2 {
		t.Fatalf("baseline = %s", data)
	}
	// The manifest has the fields of the detail of each domain event.
	var fields []string
	for _, e := range recorded.Manifest.Modules[0].Declarations[0].Events {
		for _, f := range e.Detail {
			fields = append(fields, e.Name+"."+f.Name+":"+f.Type.Text)
		}
	}
	if got := strings.Join(fields, " "); got != "cart-changed.count:number" {
		t.Errorf("detail fields = %q", got)
	}
	// The same contract passes at the same version.
	if code, text := wcCheck(t, dir); code != 0 {
		t.Fatalf("gx wc check of the baseline itself = %d: %s", code, text)
	}

	// An addition: a new attribute.
	edit(t, dir, "cart/route/route.go", "\tCompact  bool   `query:\"compact\"`\n", "\tCompact  bool   `query:\"compact\"`\n\tLocale   string `query:\"locale\"`\n")
	code, text := wcCheck(t, dir)
	if code != 1 || !strings.Contains(text, "an addition needs a minor bump") || !strings.Contains(text, "1.1.0") {
		t.Fatalf("an addition at the version of the baseline = %d: %s", code, text)
	}
	widgetsTable(t, dir, "1.0.1")
	if code, _ := wcCheck(t, dir); code != 1 {
		t.Fatalf("an addition with a patch bump = %d", code)
	}
	widgetsTable(t, dir, "1.1.0")
	if code, text := wcCheck(t, dir); code != 0 {
		t.Fatalf("an addition with a minor bump = %d: %s", code, text)
	}

	// A breaking change: a field of an event detail has a new type.
	edit(t, dir, "cart/cart.go", "Count int `json:\"count\"`", "Count string `json:\"count\"`")
	code, text = wcCheck(t, dir)
	if code != 1 || !strings.Contains(text, "a breaking change needs a major bump") || !strings.Contains(text, "2.0.0") {
		t.Fatalf("a breaking change with a minor bump = %d: %s", code, text)
	}
	// A failed check does not move the baseline, also with --update.
	if code, _ := wcCheck(t, "--update", dir); code != 1 {
		t.Fatalf("gx wc check --update with a missing bump = %d", code)
	}
	if after, err := os.ReadFile(baseline); err != nil || string(after) != string(data) {
		t.Fatalf("a failed check changed the baseline (%v)", err)
	}
	widgetsTable(t, dir, "2.0.0")
	if code, text := wcCheck(t, "--update", dir); code != 0 {
		t.Fatalf("a breaking change with a major bump = %d: %s", code, text)
	}
	// The new baseline is the contract of 2.0.0: a removed attribute is
	// breaking from there.
	edit(t, dir, "cart/route/route.go", "\tLocale   string `query:\"locale\"`\n", "")
	if code, text := wcCheck(t, dir); code != 1 || !strings.Contains(text, "3.0.0") {
		t.Fatalf("a removed attribute at 2.0.0 = %d: %s", code, text)
	}

	// A version that is not a version, and no version.
	widgetsTable(t, dir, "next")
	if code, _ := wcCheck(t, dir); code != 1 {
		t.Fatalf("gx wc check with the version next = %d", code)
	}
	if err := os.Remove(filepath.Join(dir, "gx.toml")); err != nil {
		t.Fatal(err)
	}
	if code, text := wcCheck(t, dir); code != 1 || !strings.Contains(text, "[widgets]") {
		t.Fatalf("gx wc check with no gx.toml = %d: %s", code, text)
	}
}

// TestREQ_ISL_13_WCCheckCSSVariables checks that the CSS variables of a
// widget are in its contract: the tokens of the theme that its stylesheet
// reads. A widget that stops the use of a token has a breaking change.
func TestREQ_ISL_13_WCCheckCSSVariables(t *testing.T) {
	dir := widgetModule(t)
	widgetsTable(t, dir, "1.0.0")
	_, file, _, _ := runtime.Caller(0)
	shop := filepath.Join(filepath.Dir(file), "..", "examples", "shop")
	for from, to := range map[string]string{"app/theme.css": "app/theme.css", "gx.lock": "gx.lock"} {
		data, err := os.ReadFile(filepath.Join(shop, filepath.FromSlash(from)))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, filepath.FromSlash(to))), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(to)), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	edit(t, dir, "cart/Cart.gx", "<section>", `<section class="bg-primary rounded-xl">`)
	out := filepath.Join(t.TempDir(), "widgets")
	if code := gxcli.Main([]string{"wc", "build", "--out", out, dir}); code != 0 {
		t.Fatalf("gx wc build = %d", code)
	}
	var manifest struct {
		Modules []struct {
			Declarations []struct {
				TagName       string
				CSSProperties []struct{ Name string } `json:"cssProperties"`
			}
		}
	}
	data, err := os.ReadFile(filepath.Join(out, "custom-elements.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, v := range manifest.Modules[0].Declarations[0].CSSProperties {
		names[v.Name] = true
	}
	if manifest.Modules[0].Declarations[0].TagName != "acme-cart" || !names["--primary"] || !names["--radius"] || names["--tw-shadow"] || names["--spacing"] {
		t.Fatalf("CSS variables of acme-cart = %v", names)
	}
	if code, text := wcCheck(t, "--update", dir); code != 0 {
		t.Fatalf("gx wc check --update = %d: %s", code, text)
	}
	edit(t, dir, "cart/Cart.gx", `class="bg-primary rounded-xl"`, `class="rounded-xl"`)
	code, text := wcCheck(t, dir)
	if code != 1 || !strings.Contains(text, "a breaking change needs a major bump") {
		t.Fatalf("a widget that stops the use of a token = %d: %s", code, text)
	}
}

// TestREQ_ISL_14_WCPackAndPublish checks `gx wc pack` and `gx wc publish`:
// the tarball holds the files of each widget, and the publish goes to a
// registry fixture over HTTP with the token of the environment. No node
// runs.
func TestREQ_ISL_14_WCPackAndPublish(t *testing.T) {
	// A PATH with the Go tools only: no node and no npm.
	t.Setenv("PATH", filepath.Join(runtime.GOROOT(), "bin"))
	dir := widgetModule(t)
	widgetsTable(t, dir, "1.0.0")
	if code := gxcli.Main([]string{"generate", dir}); code != 0 {
		t.Fatalf("gx generate = %d", code)
	}
	out := filepath.Join(t.TempDir(), "pack")
	if code := gxcli.Main([]string{"wc", "pack", "--out", out, dir}); code != 0 {
		t.Fatalf("gx wc pack = %d", code)
	}
	f, err := os.Open(filepath.Join(out, "acme-widgets-1.0.0.tgz"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for tr := tar.NewReader(zr); ; {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(tr)
		files[h.Name] = string(body)
	}
	for _, name := range []string{"package.json", "acme-cart.js", "acme-cart.d.ts", "acme-cart.react.d.ts", "acme-composer.js", "acme-composer.d.ts", "acme-composer.react.d.ts", "custom-elements.json"} {
		if files["package/"+name] == "" {
			t.Errorf("the tarball has no %s", name)
		}
	}
	// The element file has the server of gx.toml.
	if !strings.Contains(files["package/acme-cart.js"], `"server":"https://api.acme.dev","path":"/widgets/cart"`) {
		t.Errorf("the element file of the tarball:\n%.300s", files["package/acme-cart.js"])
	}
	if !strings.Contains(files["package/package.json"], `"name": "@acme/widgets"`) || !strings.Contains(files["package/package.json"], `"./acme-composer"`) {
		t.Errorf("package.json:\n%s", files["package/package.json"])
	}

	var puts []string
	var body struct {
		Access      string
		Attachments map[string]struct{ Length int } `json:"_attachments"`
	}
	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		puts = append(puts, r.Method+" "+r.URL.EscapedPath()+" "+r.Header.Get("Authorization"))
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer registry.Close()

	// No token: no request.
	t.Setenv("NPM_TOKEN", "")
	if code := gxcli.Main([]string{"wc", "publish", "--registry", registry.URL, dir}); code != 1 || len(puts) != 0 {
		t.Fatalf("gx wc publish with no token = %d, requests %v", code, puts)
	}
	t.Setenv("NPM_TOKEN", "token-of-ci")
	if code := gxcli.Main([]string{"wc", "publish", "--registry", registry.URL, dir}); code != 0 {
		t.Fatalf("gx wc publish = %d", code)
	}
	if len(puts) != 1 || puts[0] != "PUT /@acme%2fwidgets Bearer token-of-ci" {
		t.Fatalf("requests = %v", puts)
	}
	if body.Access != "public" || body.Attachments["acme-widgets-1.0.0.tgz"].Length == 0 {
		t.Errorf("the document of the publish = %+v", body)
	}
	// The publish records the baseline, so the next check compares with
	// this version.
	data, err := os.ReadFile(filepath.Join(dir, ".gx", "base", "widgets.json"))
	if err != nil || !strings.Contains(string(data), `"version": "1.0.0"`) {
		t.Fatalf("the baseline after the publish: %v\n%.200s", err, data)
	}
	// A breaking change with no bump stops the publish before a request.
	edit(t, dir, "cart/route/route.go", "\tCompact  bool   `query:\"compact\"`\n", "")
	if code := gxcli.Main([]string{"wc", "publish", "--registry", registry.URL, dir}); code != 1 || len(puts) != 1 {
		t.Fatalf("gx wc publish of a breaking change with no bump = %d, requests %v", code, puts)
	}
}
