// Package round4_test holds the blocking findings of review round 4 for
// release 0.2.0 (SDD §16.3). Every test fails on the reviewed HEAD 05bda5b.
package round4_test

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alternayte/gx/internal/compiler"
	"github.com/alternayte/gx/internal/devserver"
)

// repoRoot returns the root of the gx module.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

// scratchModule returns a temp module with the files. The module replaces
// gx with the repository under review.
func scratchModule(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	root := repoRoot(t)
	all := map[string]string{
		"go.mod": "module app\n\ngo 1.25.0\n\nrequire github.com/alternayte/gx v0.0.0\n\nreplace github.com/alternayte/gx => " + filepath.ToSlash(root) + "\n",
	}
	if sum, err := os.ReadFile(filepath.Join(root, "go.sum")); err == nil {
		all["go.sum"] = string(sum)
	}
	for rel, content := range files {
		all[rel] = content
	}
	for rel, content := range all {
		writeFile(t, dir, rel, content)
	}
	return dir
}

func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// generateInto runs the generator and writes its files into the module.
func generateInto(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	files, diags := compiler.Generate(dir)
	if len(diags) > 0 {
		t.Fatalf("Generate diagnostics: %v", diags)
	}
	for path, src := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, src, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return files
}

// goRun runs the main package of a scratch module and returns its output.
func goRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("go", append([]string{"run"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
	out, _ := cmd.CombinedOutput()
	return string(out)
}

// The dev app of the swap tests: one slice with a page, and a main that
// installs the dev symbol table as the scaffold of gx init does.
const (
	devRoute = "package route\n\nimport \"github.com/alternayte/gx\"\n\ntype Home struct {\n\tgx.Route `GET /`\n}\n"
	devSite  = "package site\n\nimport (\n\t\"github.com/alternayte/gx\"\n\t\"app/site/route\"\n)\n\n" +
		"var HomePage = gx.Page(\n\tfunc(c *gx.Ctx, in route.Home) (HomeProps, error) { return HomeProps{}, nil },\n\tHome)\n\n" +
		"var Routes = gx.Collect(HomePage)\n"
	devMain = "package main\n\nimport (\n\t\"log\"\n\t\"net/http\"\n\t\"os\"\n\n\t\"app/site\"\n\t\"github.com/alternayte/gx\"\n\t\"github.com/alternayte/gx/adapters/datastar\"\n)\n\n" +
		"func main() {\n\tsetupDev()\n\tapp := gx.New(gx.Config{Adapter: datastar.Adapter()})\n\tapp.Group(\"/\", site.Routes)\n\tlog.Fatal(http.ListenAndServe(os.Getenv(\"GX_DEV_ADDR\"), app))\n}\n"
	devMainDev  = "//go:build gxdev\n\npackage main\n\nimport (\n\t\"app/gxdev_symbols\"\n\t\"github.com/alternayte/gx\"\n)\n\nfunc setupDev() { gx.SetDevSymbols(gxdev_symbols.Packages()) }\n"
	devMainProd = "//go:build !gxdev\n\npackage main\n\nfunc setupDev() {}\n"
)

// devApp is a running gx dev on a scratch app.
type devApp struct {
	t    *testing.T
	dir  string
	addr string
	ch   chan devEvent
	log  *syncBuffer
}

type devEvent struct{ name, data string }

type syncBuffer struct {
	mu sync.Mutex
	b  strings.Builder
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// startDevApp writes the app with the templates, starts gx dev and waits
// for the home page to hold ready.
func startDevApp(t *testing.T, files map[string]string, ready string) *devApp {
	t.Helper()
	all := map[string]string{
		"site/route/route.go":  devRoute,
		"site/site.go":         devSite,
		"cmd/app/main.go":      devMain,
		"cmd/app/dev_gxdev.go": devMainDev,
		"cmd/app/dev_prod.go":  devMainProd,
	}
	for k, v := range files {
		all[k] = v
	}
	d := &devApp{t: t, dir: scratchModule(t, all), log: &syncBuffer{}}
	// go mod tidy gives the module the requirements of the adapter. The
	// first run is before the generator: the type check of the generator
	// needs them, and the generated symbol table package does not exist
	// yet (-e).
	first := exec.Command("go", "mod", "tidy", "-e")
	first.Dir = d.dir
	_, _ = first.CombinedOutput()
	generateInto(t, d.dir)
	tidy := exec.Command("go", "mod", "tidy")
	tidy.Dir = d.dir
	if out, err := tidy.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy: %v\n%s", err, out)
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	d.addr = l.Addr().String()
	_ = l.Close()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() {
		_ = devserver.Run(ctx, devserver.Options{Dir: d.dir, Main: "./cmd/app", Addr: d.addr, Log: d.log})
	}()
	d.waitPage(ready, 180*time.Second)
	res, err := http.Get("http://" + d.addr + "/_gx/dev")
	if err != nil {
		t.Fatal(err)
	}
	d.ch = make(chan devEvent, 64)
	t.Cleanup(func() { _ = res.Body.Close() })
	go func() {
		sc := bufio.NewScanner(res.Body)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		var e devEvent
		for sc.Scan() {
			line := sc.Text()
			switch {
			case strings.HasPrefix(line, "event: "):
				e.name = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				e.data = strings.TrimPrefix(line, "data: ")
			case line == "":
				if e.name != "" {
					d.ch <- e
				}
				e = devEvent{}
			}
		}
	}()
	// The stream is open before the first edit.
	time.Sleep(300 * time.Millisecond)
	return d
}

func (d *devApp) page() string {
	res, err := http.Get("http://" + d.addr + "/")
	if err != nil {
		return ""
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	return string(body)
}

func (d *devApp) waitPage(want string, timeout time.Duration) string {
	d.t.Helper()
	deadline := time.Now().Add(timeout)
	last := ""
	for time.Now().Before(deadline) {
		last = d.page()
		if strings.Contains(last, want) {
			return last
		}
		time.Sleep(50 * time.Millisecond)
	}
	d.t.Fatalf("timeout: the page does not contain %q; last body:\n%s\nlog:\n%s", want, last, d.log.String())
	return ""
}

func (d *devApp) pid() int {
	d.t.Helper()
	res, err := http.Get("http://" + d.addr + "/_gx/dev/info")
	if err != nil {
		d.t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		PID int `json:"pid"`
	}
	_ = json.NewDecoder(res.Body).Decode(&out)
	return out.PID
}

// next waits for the next reload or overlay event. swap is true when the
// event is a reload of a swap with no rebuild.
func (d *devApp) next(timeout time.Duration) (name string, swap bool, data string) {
	d.t.Helper()
	select {
	case e := <-d.ch:
		var r struct {
			Swap bool `json:"swap"`
		}
		_ = json.Unmarshal([]byte(e.data), &r)
		return e.name, r.Swap, e.data
	case <-time.After(timeout):
		d.t.Fatalf("no dev event in %s; log:\n%s", timeout, d.log.String())
	}
	return "", false, ""
}

// settle waits until gx dev has handled an edit: it reads the events of
// the next moments and returns them. No event is not an error.
func (d *devApp) settle(wait time.Duration) []devEvent {
	var out []devEvent
	deadline := time.After(wait)
	for {
		select {
		case e := <-d.ch:
			out = append(out, e)
		case <-deadline:
			return out
		}
	}
}

func (d *devApp) write(rel, content string) {
	d.t.Helper()
	writeFile(d.t, d.dir, rel, content)
}
