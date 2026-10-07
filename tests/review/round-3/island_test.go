package round3_test

import (
	"context"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/jspin"
)

// REQ-ISL-02: the compiler writes <Name>.props.ts from the Go struct, and
// a struct with json tags is in the mapping. The file must be TypeScript
// that the check of REQ-ISL-08 accepts.
//
// Defect: the generated file ends with fixed declarations (SignalRef,
// Signal, Ctx, Mount, Update), and the mapper reserves only the name
// Props. A Go struct with the name Update or Signal gets an interface of
// that name, so the file declares the name two times. The pinned compiler
// reports TS2300 "Duplicate identifier 'Update'" and TS2428 for Signal, in
// a file that the user cannot edit.
func TestREQ_ISL_02_PropsFileDeclaresEachNameOnce(t *testing.T) {
	dir := scratchModule(t, map[string]string{
		"feed/Timeline.ts": "import type { Props } from \"./Timeline.props\";\n\nexport default (el: HTMLElement, props: Props) => {\n  el.textContent = String(props.updates.length);\n};\n",
		"feed/props.go": "package feed\n\n" +
			"// Update is one row of the timeline.\n" +
			"type Update struct {\n\tText string `json:\"text\"`\n}\n\n" +
			"// Signal is the strength of a source.\n" +
			"type Signal struct {\n\tLevel int `json:\"level\"`\n}\n\n" +
			"type TimelineProps struct {\n\tUpdates []Update `json:\"updates\"`\n\tSignal  Signal   `json:\"signal\"`\n}\n",
	})
	files := generateInto(t, dir)
	ts := string(files[filepath.Join(dir, "feed", "Timeline.props.ts")])
	if ts == "" {
		t.Fatal("no Timeline.props.ts in the generated files")
	}
	decl := regexp.MustCompile(`(?m)^(?:export )?(?:interface|type) ([A-Za-z_$][A-Za-z0-9_$]*)`)
	count := map[string]int{}
	for _, m := range decl.FindAllStringSubmatch(ts, -1) {
		count[m[1]]++
	}
	for _, name := range []string{"Update", "Signal"} {
		if n := count[name]; n != 1 {
			t.Errorf("Timeline.props.ts declares the name %s %d times, want 1; the TypeScript check refuses the file", name, n)
		}
	}
	if t.Failed() {
		t.Logf("Timeline.props.ts:\n%s", ts)
	}
}

// REQ-ISL-07: gx pin stores the build in the vendor directory of the app
// (js/vendor/, B-007) and records it in gx.lock.
//
// Defect: gx pin takes the path of each file from the text of the module
// that it fetched. An import address with ".." parts, for example
// "/npm/dep@1.0.0/../../../../stolen/+esm", gives the file
// js/vendor/dep@1.0.0/../../../../stolen.js, and the command writes it
// outside the app. The text of a module is the text of an npm package or of
// a mirror, so a package can write a file with its own content to a
// directory above the app. The same path reaches jspin.Pin from gx wc pin
// through the module path of a custom elements manifest.
func TestREQ_ISL_07_PinWritesOnlyInTheVendorDirectory(t *testing.T) {
	// A CDN that reads ".." as a server does: it answers the address of
	// the module that the cleaned path names.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		if r.URL.Path == "/npm/chartlib@1.2.0/+esm" {
			_, _ = w.Write([]byte("import \"/npm/dep@1.0.0/../../../../stolen/+esm\";export default 1;\n"))
			return
		}
		_, _ = w.Write([]byte("globalThis.stolen=true;\n"))
	}))
	defer srv.Close()

	parent := t.TempDir()
	app := filepath.Join(parent, "app")
	if err := os.MkdirAll(app, 0o755); err != nil {
		t.Fatal(err)
	}
	// The pin can fail, or it can keep the file in the vendor directory.
	// It must not write outside that directory.
	_, pinErr := jspin.Pin(context.Background(), jspin.Options{Dir: app, Spec: "chartlib@1.2.0", BaseURL: srv.URL})

	vendor := filepath.Join(app, filepath.FromSlash(jspin.VendorDir)) + string(filepath.Separator)
	lock := filepath.Join(app, "gx.lock")
	err := filepath.WalkDir(parent, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if path != lock && !strings.HasPrefix(path, vendor) {
			rel, _ := filepath.Rel(app, path)
			t.Errorf("gx pin wrote %s, which is outside %s (pin error: %v)", filepath.ToSlash(rel), jspin.VendorDir, pinErr)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
