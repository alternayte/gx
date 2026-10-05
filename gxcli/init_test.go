package gxcli_test

import (
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/alternayte/gx/gxcli"
	"github.com/alternayte/gx/internal/execname"
)

// thisRepo returns the root of this repository.
func thisRepo(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), ".."))
}

// initApp scaffolds an app into a new directory with `gx init`.
func initApp(t *testing.T, extra ...string) string {
	t.Helper()
	t.Setenv("GOFLAGS", "-mod=mod")
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(parent, "acme")
	args := append([]string{"init", "--adapter", "datastar", "--module", "example.com/acme", "--replace", thisRepo(t)}, extra...)
	args = append(args, dir)
	out, code := captureStdout(t, func() int { return gxcli.Main(args) })
	if code != 0 {
		t.Fatalf("gx init exit = %d\n%s", code, out)
	}
	return dir
}

// treeOf lists the files of dir, sorted, one per line.
func treeOf(t *testing.T, dir string) string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		if d.IsDir() {
			if rel == ".gx" {
				return fs.SkipDir
			}
			return nil
		}
		if rel == "go.sum" {
			return nil
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	return strings.Join(files, "\n") + "\n"
}

// goIn runs one go command in dir.
func goIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// servePage builds the app of dir, runs it and returns the body of path.
func servePage(t *testing.T, dir, path string) string {
	t.Helper()
	bin := execname.Name(filepath.Join(t.TempDir(), "app"))
	goIn(t, dir, "build", "-o", bin, "./cmd/app")
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	cmd := exec.Command(bin)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GX_DEV_ADDR="+addr)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()
	deadline := time.Now().Add(20 * time.Second)
	for {
		resp, err := http.Get("http://" + addr + path)
		if err == nil {
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("GET %s = %d\n%s", path, resp.StatusCode, body)
			}
			return string(body)
		}
		if time.Now().After(deadline) {
			t.Fatalf("GET %s: %v", path, err)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// TestREQ_DEV_10_Init covers `gx init`: the scaffold holds the project CLI,
// AGENTS.md, the theme, gx.toml and an example slice; it passes gx check,
// builds with the Go tool alone and serves its page (REQ-DEV-10).
func TestREQ_DEV_10_Init(t *testing.T) {
	dir := initApp(t)
	snapshot(t, "dev10_init_files.golden.txt", treeOf(t, dir))

	read := func(rel string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	for rel, wants := range map[string][]string{
		"cmd/gx/main.go":      {"gxcli.Main(os.Args[1:])"},
		"cmd/app/main.go":     {"datastar.Adapter()", "home.Routes", "gxstyles.CSS()"},
		"gx.toml":             {"[registry]", "[site]"},
		"app/theme.css":       {"@theme", "--background", "--primary"},
		"AGENTS.md":           {"gx check", "gx dev"},
		"go.mod":              {"module example.com/acme", "github.com/alternayte/gx"},
		"home/route/route.go": {"gx.Route `GET /{$}`"},
		"home/Home.gx":        {"<Counter"},
	} {
		body := read(rel)
		for _, want := range wants {
			if !strings.Contains(body, want) {
				t.Fatalf("%s lacks %q:\n%s", rel, want, body)
			}
		}
	}

	if out := captureStderr(t, func() {
		if code := gxcli.Main([]string{"check", dir}); code != 0 {
			t.Errorf("gx check on the scaffold exit = %d", code)
		}
	}); t.Failed() {
		t.Fatal(out)
	}
	// The Go tool alone builds the scaffold: generated code is in place.
	goIn(t, dir, "build", "./...")
	page := servePage(t, dir, "/")
	for _, want := range []string{"<!doctype html>", "Welcome to acme", "/_gx/datastar.js", "data-signals"} {
		if !strings.Contains(page, want) {
			t.Fatalf("the home page lacks %q:\n%s", want, page)
		}
	}

	// A second init must not write into an app.
	before := read("home/Home.gx")
	if err := os.WriteFile(filepath.Join(dir, "home/Home.gx"), []byte(before+"\n<p>mine</p>\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stderr := captureStderr(t, func() {
		if code := gxcli.Main([]string{"init", "--adapter", "datastar", dir}); code == 0 {
			t.Error("gx init wrote into a directory that holds an app")
		}
	})
	if !strings.Contains(stderr, "go.mod") {
		t.Fatalf("the refusal does not name the reason:\n%s", stderr)
	}
	if !strings.Contains(read("home/Home.gx"), "<p>mine</p>") {
		t.Fatal("gx init changed a file of the app")
	}
}

// withStdin runs fn with os.Stdin set to the given text.
func withStdin(t *testing.T, text string, fn func()) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.WriteString(text); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()
	old := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = old }()
	fn()
}

// TestREQ_DEV_10_InitAsksForAdapter covers the adapter question: an empty
// answer takes Datastar, and htmx is refused until release 0.2.0
// (REQ-DEV-10, DR-04).
func TestREQ_DEV_10_InitAsksForAdapter(t *testing.T) {
	t.Setenv("GOFLAGS", "-mod=mod")
	parent := t.TempDir()
	base := []string{"init", "--module", "example.com/asked", "--replace", thisRepo(t)}

	var out string
	var code int
	withStdin(t, "\n", func() {
		out, code = captureStdout(t, func() int { return gxcli.Main(append(base, filepath.Join(parent, "asked"))) })
	})
	if code != 0 || !strings.Contains(out, "Adapter [datastar]") {
		t.Fatalf("gx init with an empty answer: exit %d\n%s", code, out)
	}
	main, err := os.ReadFile(filepath.Join(parent, "asked", "cmd", "app", "main.go"))
	if err != nil || !strings.Contains(string(main), "datastar.Adapter()") {
		t.Fatalf("the scaffold does not use Datastar: %v\n%s", err, main)
	}

	var stderr string
	withStdin(t, "htmx\n", func() {
		stderr = captureStderr(t, func() {
			_, code = captureStdout(t, func() int { return gxcli.Main(append(base, filepath.Join(parent, "later"))) })
		})
	})
	if code == 0 || !strings.Contains(stderr, "0.2.0") {
		t.Fatalf("gx init with htmx: exit %d\n%s", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(parent, "later")); err == nil {
		t.Fatal("a refused init wrote files")
	}
}

// TestREQ_DEV_10_New covers `gx new`: a slice, a page, an action, a form
// and a component each give typed code that passes gx check, builds and is
// mounted (REQ-DEV-10).
func TestREQ_DEV_10_New(t *testing.T) {
	dir := initApp(t)
	for _, args := range [][]string{
		{"new", "slice", "shop", dir},
		{"new", "page", "shop/Detail", dir},
		{"new", "action", "shop/AddItem", dir},
		{"new", "form", "shop/Contact", dir},
		{"new", "component", "ui/badge/Badge", dir},
	} {
		out, code := captureStdout(t, func() int { return gxcli.Main(args) })
		if code != 0 {
			t.Fatalf("gx %s exit = %d\n%s", strings.Join(args[:3], " "), code, out)
		}
	}
	for _, rel := range []string{
		"shop/route/route.go", "shop/shop.go", "shop/IndexView.gx",
		"shop/detail_page.go", "shop/DetailView.gx",
		"shop/add_item_action.go",
		"shop/contact_form.go", "shop/ContactView.gx",
		"ui/badge/Badge.gx", "ui/badge/Badge.fixtures.go",
	} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("gx new did not write %s", rel)
		}
	}
	if out := captureStderr(t, func() {
		if code := gxcli.Main([]string{"check", dir}); code != 0 {
			t.Errorf("gx check after gx new exit = %d", code)
		}
	}); t.Failed() {
		t.Fatal(out)
	}
	goIn(t, dir, "build", "./...")

	// The new slice is mounted, and its pages answer.
	for path, want := range map[string]string{
		"/shop":         "Shop",
		"/shop/detail":  "Detail",
		"/shop/contact": `data-gx-form="contact"`,
	} {
		if page := servePage(t, dir, path); !strings.Contains(page, want) {
			t.Fatalf("GET %s lacks %q:\n%s", path, want, page)
		}
	}

	// Bad names and existing files are refused with nothing written.
	for _, args := range [][]string{
		{"new", "page", "shop/detail", dir},         // a component name is exported
		{"new", "page", "shop/Detail", dir},         // exists
		{"new", "slice", "Shop", dir},               // a package name is lower case
		{"new", "widget", "shop/Thing", dir},        // unknown kind
		{"new", "action", "missing/Thing", dir},     // no such slice
		{"new", "component", "ui/badge/Badge", dir}, // exists
	} {
		stderr := captureStderr(t, func() {
			if _, code := captureStdout(t, func() int { return gxcli.Main(args) }); code == 0 {
				t.Errorf("gx %s passed", strings.Join(args[:3], " "))
			}
		})
		if strings.TrimSpace(stderr) == "" {
			t.Errorf("gx %s failed with no message", strings.Join(args[:3], " "))
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("a refused gx new wrote files")
	}
}
