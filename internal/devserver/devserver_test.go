package devserver_test

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/alternayte/gx/internal/devserver"
	"github.com/alternayte/gx/internal/execname"
	"github.com/alternayte/gx/internal/testbudget"
)

// appMain is the stdlib-only test app. The dev loop needs no gx import.
const appMain = `package main

import (
	"fmt"
	"net/http"
	"os"
)

func main() {
	addr := os.Getenv("GX_DEV_ADDR")
	if addr == "" {
		addr = "127.0.0.1:18999"
	}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "<html><head><title>app</title></head><body>__VERSION__</body></html>")
	})
	_ = http.ListenAndServe(addr, nil)
}
`

// gxAppMain imports package gx so the dev route build tag is exercised.
const gxAppMain = `package main

import (
	"net/http"
	"os"

	"github.com/alternayte/gx"
)

func main() {
	app := gx.New(gx.Config{})
	_ = http.ListenAndServe(os.Getenv("GX_DEV_ADDR"), app)
}
`

// writeApp writes a stdlib app module with one version string.
func writeApp(t *testing.T, version string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module app\n\ngo 1.25.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "cmd", "app"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeVersion(t, dir, version)
	return dir
}

func writeVersion(t *testing.T, dir, version string) {
	t.Helper()
	src := strings.Replace(appMain, "__VERSION__", version, 1)
	if err := os.WriteFile(filepath.Join(dir, "cmd", "app", "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
}

// freeAddr returns a free local address.
func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().String()
}

// startDev runs the dev loop until stop.
func startDev(t *testing.T, dir string) (addr string, stop func()) {
	t.Helper()
	addr = freeAddr(t)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		if err := devserver.Run(ctx, devserver.Options{Dir: dir, Main: "./cmd/app", Addr: addr, Log: os.Stderr}); err != nil {
			t.Logf("devserver: %v", err)
		}
	}()
	stop = cancel
	t.Cleanup(cancel)
	return addr, stop
}

// getText polls a URL until its body contains want.
func getText(t *testing.T, url, want string, timeout time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	last := ""
	for time.Now().Before(deadline) {
		res, err := http.Get(url)
		if err == nil {
			body, _ := io.ReadAll(res.Body)
			_ = res.Body.Close()
			last = string(body)
			if strings.Contains(last, want) {
				return last
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timeout: %s does not contain %q; last body %q", url, want, last)
	return ""
}

// event is one SSE event of the dev channel.
type event struct {
	name string
	data string
}

// events connects to the dev event stream.
func events(t *testing.T, addr string) chan event {
	t.Helper()
	ch := make(chan event, 32)
	res, err := http.Get("http://" + addr + "/_gx/dev")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		defer res.Body.Close()
		sc := bufio.NewScanner(res.Body)
		var name, data string
		for sc.Scan() {
			line := sc.Text()
			switch {
			case strings.HasPrefix(line, "event:"):
				name = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			case strings.HasPrefix(line, "data:"):
				data += strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			case line == "":
				if name != "" {
					ch <- event{name: name, data: data}
				}
				name, data = "", ""
			}
		}
		close(ch)
	}()
	return ch
}

// waitEvent waits for one named event.
func waitEvent(t *testing.T, ch chan event, name string, timeout time.Duration) event {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case e, ok := <-ch:
			if !ok {
				t.Fatalf("event stream closed while waiting for %s", name)
			}
			if e.name == name {
				return e
			}
		case <-deadline:
			t.Fatalf("timeout waiting for %s", name)
		}
	}
}

// TestREQ_DEV_01_DevServer covers one command and a reachable app with the
// injected dev client (REQ-DEV-01).
func TestREQ_DEV_01_DevServer(t *testing.T) {
	dir := writeApp(t, "v1")
	addr, _ := startDev(t, dir)
	body := getText(t, "http://"+addr+"/", "v1", 60*time.Second)
	if !strings.Contains(body, "/_gx/dev-client.js") {
		t.Fatalf("the page lacks the dev client:\n%s", body)
	}
	res, err := http.Get("http://" + addr + "/_gx/dev-client.js")
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("dev client = %d, want 200", res.StatusCode)
	}
}

// TestREQ_DEV_03_RebuildAndMorph covers a Go change: rebuild, restart and a
// reload event for the morph (REQ-DEV-03).
func TestREQ_DEV_03_RebuildAndMorph(t *testing.T) {
	dir := writeApp(t, "v1")
	addr, _ := startDev(t, dir)
	getText(t, "http://"+addr+"/", "v1", 60*time.Second)
	ch := events(t, addr)
	writeVersion(t, dir, "v2")
	waitEvent(t, ch, "reload", 20*time.Second)
	getText(t, "http://"+addr+"/", "v2", 20*time.Second)
}

// TestREQ_DEV_06_ErrorOverlay covers a build error overlay and the recovery
// after the fix (REQ-DEV-06).
func TestREQ_DEV_06_ErrorOverlay(t *testing.T) {
	dir := writeApp(t, "v1")
	addr, _ := startDev(t, dir)
	getText(t, "http://"+addr+"/", "v1", 60*time.Second)
	ch := events(t, addr)
	bad := "package main\n\nfunc main() { this is not go }\n"
	if err := os.WriteFile(filepath.Join(dir, "cmd", "app", "main.go"), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	overlay := waitEvent(t, ch, "overlay", 20*time.Second)
	if !strings.Contains(overlay.data, "build failed") {
		t.Fatalf("overlay = %s", overlay.data)
	}
	if !strings.Contains(overlay.data, "vscode://file/") {
		t.Fatalf("overlay lacks an editor link: %s", overlay.data)
	}
	writeVersion(t, dir, "v3")
	waitEvent(t, ch, "reload", 20*time.Second)
	getText(t, "http://"+addr+"/", "v3", 20*time.Second)
}

// TestNFR_02_GoChangeUnder2s measures save to reload (NFR-02).
func TestNFR_02_GoChangeUnder2s(t *testing.T) {
	dir := writeApp(t, "v1")
	addr, _ := startDev(t, dir)
	getText(t, "http://"+addr+"/", "v1", 60*time.Second)
	ch := events(t, addr)
	start := time.Now()
	writeVersion(t, dir, "v2")
	waitEvent(t, ch, "reload", 20*time.Second)
	took := time.Since(start)
	t.Logf("NFR-02: save to reload %s", took)
	if budget := testbudget.Budget(2500*time.Millisecond, 8); took > budget {
		t.Fatalf("NFR-02: save to reload %s, want under %s on this machine", took, budget)
	}
}

// repoRoot returns the module root of the repo.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

// buildGxApp builds a gx app module with the given tags and returns the
// binary. It skips when the Go toolchain cannot build it.
func buildGxApp(t *testing.T, tags string) string {
	t.Helper()
	dir := t.TempDir()
	mod := "module app\n\ngo 1.25.0\n\nrequire github.com/alternayte/gx v0.0.0\n\nreplace github.com/alternayte/gx => " +
		filepath.ToSlash(repoRoot(t)) + "\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(mod), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "cmd", "app"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cmd", "app", "main.go"), []byte(gxAppMain), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := execname.Name(filepath.Join(t.TempDir(), "app"))
	args := []string{"build", "-o", bin}
	if tags != "" {
		args = append(args, "-tags", tags)
	}
	args = append(args, "./cmd/app")
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build %s: %v\n%s", tags, err, out)
	}
	return bin
}

// TestREQ_DEV_07_DevRoutes covers the dev-only route under the gxdev tag
// (REQ-DEV-07, SI-08).
func TestREQ_DEV_07_DevRoutes(t *testing.T) {
	run := func(tags string) *http.Response {
		t.Helper()
		bin := buildGxApp(t, tags)
		addr := freeAddr(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		cmd := exec.CommandContext(ctx, bin)
		cmd.Env = append(os.Environ(), "GX_DEV_ADDR="+addr)
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		}()
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			res, err := http.Get("http://" + addr + "/_gx/dev/info")
			if err == nil {
				return res
			}
			time.Sleep(50 * time.Millisecond)
		}
		t.Fatal("app did not start")
		return nil
	}
	dev := run("gxdev")
	_ = dev.Body.Close()
	if dev.StatusCode != http.StatusOK {
		t.Fatalf("dev build /_gx/dev/info = %d, want 200", dev.StatusCode)
	}
	prod := run("")
	_ = prod.Body.Close()
	if prod.StatusCode != http.StatusNotFound {
		t.Fatalf("prod build /_gx/dev/info = %d, want 404", prod.StatusCode)
	}
}

// The dev-only parts that SI-08 names. A symbol of the binary that holds
// one of devSymbols belongs to the dev symbol table of the app, to the
// interpreter, or to the part of package gx that installs and runs them. A
// string of devStrings is the path of a dev-only route.
var (
	devSymbols = []string{
		"/gxdev_symbols.",
		"/gxdev_gallery.",
		"github.com/alternayte/gx/internal/interp.",
		"github.com/alternayte/gx.SetDevSymbols",
		"github.com/alternayte/gx.DevSwap",
		"github.com/alternayte/gx.devSwapAllowed",
		"github.com/alternayte/gx.watchParent",
	}
	devStrings = []string{
		"/_gx/dev/swap",
		"/_gx/dev/info",
		"/_gx/gallery",
		"/_gx/export",
	}
)

// buildShop builds the example shop and returns the binary. The shop has
// templates, a generated symbol table and the dev and production pair of
// main files that the scaffold writes.
func buildShop(t *testing.T, tags string) string {
	t.Helper()
	bin := execname.Name(filepath.Join(t.TempDir(), "shop"))
	args := []string{"build", "-o", bin}
	if tags != "" {
		args = append(args, "-tags", tags)
	}
	cmd := exec.Command("go", append(args, "./cmd/shop")...)
	cmd.Dir = filepath.Join(repoRoot(t), "examples", "shop")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build the shop with tags %q: %v\n%s", tags, err, out)
	}
	return bin
}

// devParts returns the dev-only symbols and strings that a binary holds.
func devParts(t *testing.T, bin string) (symbols, strs []string) {
	t.Helper()
	out, err := exec.Command("go", "tool", "nm", bin).Output()
	if err != nil {
		t.Fatalf("go tool nm: %v", err)
	}
	if lines := bytes.Count(out, []byte("\n")); lines < 1000 {
		t.Fatalf("go tool nm lists %d symbols; the binary has no symbol table to scan", lines)
	}
	for _, marker := range devSymbols {
		if bytes.Contains(out, []byte(marker)) {
			symbols = append(symbols, marker)
		}
	}
	data, err := os.ReadFile(bin)
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range devStrings {
		if bytes.Contains(data, []byte(marker)) {
			strs = append(strs, marker)
		}
	}
	return symbols, strs
}

// TestSI_08_ProdBinary proves that a production binary of an app holds no
// dev symbol table, no interpreter and no dev-only route (SI-08). It scans
// the symbols and the strings of the binary, and it sends requests to the
// dev-only routes of the running app. The same scan of a dev build finds
// each part, so a scan that finds none is a real result.
func TestSI_08_ProdBinary(t *testing.T) {
	prod := buildShop(t, "")
	symbols, strs := devParts(t, prod)
	if len(symbols) != 0 {
		t.Errorf("the production binary holds the dev symbols %q", symbols)
	}
	if len(strs) != 0 {
		t.Errorf("the production binary holds the dev strings %q", strs)
	}

	dev := buildShop(t, "gxdev")
	symbols, strs = devParts(t, dev)
	if len(symbols) != len(devSymbols) {
		t.Errorf("the scan of the dev binary finds the symbols %q, want each of %q", symbols, devSymbols)
	}
	if len(strs) != len(devStrings) {
		t.Errorf("the scan of the dev binary finds the strings %q, want each of %q", strs, devStrings)
	}

	// The routes: a production build of an app answers 404 on each dev-only
	// route, and a dev build of the same app has the route. The app has no
	// route of its own, so the answer comes from the framework.
	start := func(tags string) string {
		t.Helper()
		addr := freeAddr(t)
		ctx, cancel := context.WithCancel(context.Background())
		cmd := exec.CommandContext(ctx, buildGxApp(t, tags))
		// A secret of a gx dev does not make the swap route exist.
		cmd.Env = append(os.Environ(), "GX_DEV_ADDR="+addr, "GX_DEV_SECRET=0123456789abcdef")
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			cancel()
			_ = cmd.Wait()
		})
		for deadline := time.Now().Add(15 * time.Second); ; time.Sleep(50 * time.Millisecond) {
			if conn, err := net.Dial("tcp", addr); err == nil {
				_ = conn.Close()
				return "http://" + addr
			}
			if time.Now().After(deadline) {
				t.Fatalf("the app with tags %q did not start", tags)
			}
		}
	}
	status := func(method, url string) int {
		t.Helper()
		req, err := http.NewRequest(method, url, strings.NewReader(`{"package":"x","file":"x_gx.go","source":"package x"}`))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Gx-Dev-Secret", "0123456789abcdef")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
		return res.StatusCode
	}
	prodApp, devApp := start(""), start("gxdev")
	for _, route := range []struct{ method, path string }{
		{http.MethodPost, "/_gx/dev/swap"},
		{http.MethodGet, "/_gx/dev/info"},
		{http.MethodGet, "/_gx/gallery"},
		{http.MethodGet, "/_gx/export"},
	} {
		if got := status(route.method, prodApp+route.path); got != http.StatusNotFound {
			t.Errorf("%s %s of the production build = %d, want 404", route.method, route.path, got)
		}
		if got := status(route.method, devApp+route.path); got == http.StatusNotFound || got == http.StatusMethodNotAllowed {
			t.Errorf("%s %s of the dev build = %d, want the route", route.method, route.path, got)
		}
	}
}
