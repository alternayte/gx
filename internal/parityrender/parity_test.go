package parityrender

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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

	"github.com/alternayte/gx/internal/compiler"
	"github.com/alternayte/gx/internal/execname"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

// devSecret is the secret between the test and the app, as between gx dev
// and the app.
const devSecret = "0123456789abcdef0123456789abcdef"

// startDevApp builds the app of dir with the gxdev tag and runs it with the
// secret of the test.
func startDevApp(t *testing.T, dir, mainPkg string) string {
	t.Helper()
	return startDevAppEnv(t, dir, mainPkg, "GX_DEV_SECRET="+devSecret)
}

func startDevAppEnv(t *testing.T, dir, mainPkg string, env ...string) string {
	t.Helper()
	bin := execname.Name(filepath.Join(t.TempDir(), "app"))
	build := exec.Command("go", "build", "-tags", "gxdev", "-o", bin, mainPkg)
	build.Dir = dir
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build -tags gxdev: %v\n%s", err, out)
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	ctx, cancel := context.WithCancel(context.Background())
	app := exec.CommandContext(ctx, bin, "-addr", addr)
	app.Dir = dir
	// The environment of the test has no secret of a gx dev.
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "GX_DEV_SECRET=") {
			app.Env = append(app.Env, kv)
		}
	}
	app.Env = append(app.Env, "GX_DEV_ADDR="+addr)
	app.Env = append(app.Env, env...)
	if err := app.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		_ = app.Wait()
	})
	base := "http://" + addr
	for deadline := time.Now().Add(30 * time.Second); ; {
		if res, err := http.Get(base + "/_gx/dev/info"); err == nil {
			_ = res.Body.Close()
			return base
		}
		if time.Now().After(deadline) {
			t.Fatalf("the app did not start at %s", base)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func get(t *testing.T, url string) (int, []byte) {
	t.Helper()
	res, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return res.StatusCode, body
}

// TestREQ_DEV_05_ShopRendersTheSameInterpreted renders every page of the
// example shop and every fixture of its gallery two times: with the
// compiled code, and after a swap of each generated template file to
// interpreted code. The bytes are equal.
func TestREQ_DEV_05_ShopRendersTheSameInterpreted(t *testing.T) {
	shop := filepath.Join(repoRoot(t), "examples", "shop")
	files, diags := compiler.Generate(shop)
	if len(diags) != 0 {
		t.Fatalf("generate: %v", diags)
	}
	base := startDevApp(t, shop, "./cmd/shop")

	// The pages: each route with no parameter, and the gallery, which
	// renders each fixture of each component.
	_, manifest := get(t, base+"/_gx/export")
	var export struct {
		Pages []string `json:"paths"`
	}
	if err := json.Unmarshal(manifest, &export); err != nil {
		t.Fatalf("the export manifest: %v\n%s", err, manifest)
	}
	pages := append([]string{"/_gx/gallery"}, export.Pages...)
	sort.Strings(pages)
	if len(pages) < 5 {
		t.Fatalf("pages = %v", pages)
	}
	compiled := map[string][]byte{}
	for _, page := range pages {
		status, body := get(t, base+page)
		if status != http.StatusOK {
			t.Fatalf("GET %s = %d", page, status)
		}
		compiled[page] = body
	}

	// Swap each template file. A file that the interpreter does not cover
	// stays compiled; the test lists those.
	var paths []string
	for path := range files {
		if strings.HasSuffix(path, "_gx.go") && fileExists(strings.TrimSuffix(path, "_gx.go")+".gx") {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	swapped, refused := 0, map[string]int{}
	var refusals []string
	for _, path := range paths {
		rel, err := filepath.Rel(shop, filepath.Dir(path))
		if err != nil {
			t.Fatal(err)
		}
		pkg := "github.com/alternayte/gx/examples/shop"
		if rel != "." {
			pkg += "/" + filepath.ToSlash(rel)
		}
		ok, why := swap(t, base, pkg, filepath.Base(path), files[path])
		if ok {
			swapped++
			continue
		}
		reason := why
		if i := strings.Index(reason, ": "); i >= 0 {
			reason = reason[i+2:]
		}
		refused[reason]++
		refusals = append(refusals, fmt.Sprintf("%s: %s", filepath.Base(path), why))
	}
	t.Logf("%d pages; %d template files run interpreted, %d stay compiled", len(pages), swapped, len(refusals))
	for reason, n := range refused {
		t.Logf("  %d x %s", n, reason)
	}
	// The interpreter covers the code that the generator writes for the
	// templates of the shop: nine files in ten at least.
	if swapped*10 < len(paths)*9 {
		t.Errorf("only %d of %d template files run interpreted:\n%s", swapped, len(paths), strings.Join(refusals, "\n"))
	}

	for _, page := range pages {
		status, body := get(t, base+page)
		if status != http.StatusOK {
			t.Errorf("GET %s with interpreted templates = %d\n%s", page, status, firstLines(body))
			continue
		}
		if !bytes.Equal(body, compiled[page]) {
			t.Errorf("%s differs between compiled and interpreted templates:\n%s", page, firstDifference(compiled[page], body))
		}
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func firstLines(body []byte) string {
	if len(body) > 600 {
		body = body[:600]
	}
	return string(body)
}

// firstDifference shows the text around the first byte that differs.
func firstDifference(a, b []byte) string {
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	from := max(i-200, 0)
	cut := func(s []byte) string { return string(s[from:min(i+200, len(s))]) }
	return fmt.Sprintf("at byte %d of %d and %d\ncompiled:    ...%s\ninterpreted: ...%s", i, len(a), len(b), cut(a), cut(b))
}

// swap posts new generated code of one file to the running app.
func swap(t *testing.T, base, pkg, file string, src []byte) (bool, string) {
	t.Helper()
	payload, _ := json.Marshal(map[string]string{"package": pkg, "file": file, "source": string(src)})
	res := postSwap(t, base, payload, devSecret, "")
	defer res.Body.Close()
	var answer struct {
		Swapped bool
		Reason  string
	}
	if err := json.NewDecoder(res.Body).Decode(&answer); err != nil {
		t.Fatal(err)
	}
	return answer.Swapped, answer.Reason
}

// postSwap sends one swap request. An empty secret sends no secret header;
// a non-empty host is the Host header of the request.
func postSwap(t *testing.T, base string, payload []byte, secret, host string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, base+"/_gx/dev/swap", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if secret != "" {
		req.Header.Set("Gx-Dev-Secret", secret)
	}
	if host != "" {
		req.Host = host
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// aboutHeading and aboutTitle are two places of the generated code of the
// About page of the shop: the text of its heading in a static string, and
// the expression of its title.
const (
	aboutHeading = ">About</h1>"
	aboutTitle   = `Title: "Gx shop about"`
)

// TestREQ_DEV_02_SwapOnlyFromGxDev proves that the app runs new code only
// for gx dev: a swap request needs the secret that gx dev gave the app, and
// a Host header of the loopback interface (F-51). A page of a different
// site cannot send either: it does not know the secret, and after DNS
// rebinding its requests carry the name of its own host.
func TestREQ_DEV_02_SwapOnlyFromGxDev(t *testing.T) {
	shop := filepath.Join(repoRoot(t), "examples", "shop")
	const pkg = "github.com/alternayte/gx/examples/shop"
	src, err := os.ReadFile(filepath.Join(shop, "About_gx.go"))
	if err != nil {
		t.Fatal(err)
	}
	// The text of the page is in a static string of a template (DR-11).
	if !bytes.Contains(src, []byte(aboutHeading)) {
		t.Fatal("About_gx.go has no heading text in a static string")
	}
	edited := bytes.Replace(src, []byte(aboutHeading), []byte(">FOREIGN-CODE</h1>"), 1)
	payload, _ := json.Marshal(map[string]string{"package": pkg, "file": "About_gx.go", "source": string(edited)})

	base := startDevApp(t, shop, "./cmd/shop")
	port := base[strings.LastIndex(base, ":"):]
	refused := []struct{ name, secret, host string }{
		{"no secret", "", ""},
		{"a wrong secret", "ffffffffffffffffffffffffffffffff", ""},
		{"a prefix of the secret", devSecret[:16], ""},
		{"the secret with the host of a different site", devSecret, "attacker.example" + port},
		{"the secret with a host that starts with the loopback address", devSecret, "127.0.0.1.attacker.example" + port},
	}
	for _, c := range refused {
		res := postSwap(t, base, payload, c.secret, c.host)
		_ = res.Body.Close()
		if res.StatusCode != http.StatusForbidden {
			t.Errorf("%s: status = %d, want 403", c.name, res.StatusCode)
		}
	}
	if _, body := get(t, base+"/about"); bytes.Contains(body, []byte("FOREIGN-CODE")) {
		t.Fatal("a refused request changed the code of the app")
	}
	for _, host := range []string{"", "localhost" + port, "[::1]" + port} {
		res := postSwap(t, base, payload, devSecret, host)
		_ = res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Errorf("the secret with the host %q: status = %d, want 200", host, res.StatusCode)
		}
	}

	// An app that gx dev did not start has no secret. It refuses each swap,
	// also one with an empty secret header.
	alone := startDevAppEnv(t, shop, "./cmd/shop")
	for _, secret := range []string{"", devSecret} {
		res := postSwap(t, alone, payload, secret, "")
		_ = res.Body.Close()
		if res.StatusCode != http.StatusForbidden {
			t.Errorf("an app with no secret, header %q: status = %d, want 403", secret, res.StatusCode)
		}
	}
	req, _ := http.NewRequest(http.MethodPost, alone+"/_gx/dev/swap", bytes.NewReader(payload))
	req.Header.Set("Gx-Dev-Secret", "")
	if res, err := http.DefaultClient.Do(req); err != nil {
		t.Fatal(err)
	} else {
		_ = res.Body.Close()
		if res.StatusCode != http.StatusForbidden {
			t.Errorf("an app with no secret, empty header: status = %d, want 403", res.StatusCode)
		}
	}
}

// TestREQ_DEV_05_SwappedCodeIsWhatRenders proves that the parity test
// compares two different paths: after a swap, the page shows the text of
// the swapped code, and a refused swap leaves the page as it was.
func TestREQ_DEV_05_SwappedCodeIsWhatRenders(t *testing.T) {
	shop := filepath.Join(repoRoot(t), "examples", "shop")
	base := startDevApp(t, shop, "./cmd/shop")
	const pkg = "github.com/alternayte/gx/examples/shop"
	src, err := os.ReadFile(filepath.Join(shop, "About_gx.go"))
	if err != nil {
		t.Fatal(err)
	}
	_, before := get(t, base+"/about")
	if bytes.Contains(before, []byte("SWAPPED-TEXT")) {
		t.Fatal("the page has the marker before the swap")
	}
	// The generated code has the page text in the static strings of a
	// template (DR-11). This is what a markup edit changes. An expression
	// is a dynamic value of the template value.
	if !bytes.Contains(src, []byte(aboutHeading)) || !bytes.Contains(src, []byte(aboutTitle)) {
		t.Fatal("About_gx.go has no heading text in a static string, or no title expression")
	}
	edited := bytes.Replace(src, []byte(aboutHeading), []byte(">SWAPPED-TEXT</h1>"), 1)
	edited = bytes.Replace(edited, []byte(aboutTitle), []byte(`Title: "t" + strconv.Itoa(len(p.GxNoSuchField))`), 1)
	ok, reason := swap(t, base, pkg, "About_gx.go", edited)
	if ok || !strings.Contains(reason, "the symbol table has no name strconv") {
		t.Fatalf("a swap that uses a package outside the imports of the file = %v, %q", ok, reason)
	}
	if _, after := get(t, base+"/about"); !bytes.Equal(after, before) {
		t.Fatal("a refused swap changed the page")
	}

	edited = bytes.Replace(src, []byte(aboutHeading), []byte(">SWAPPED-TEXT</h1>"), 1)
	if ok, reason := swap(t, base, pkg, "About_gx.go", edited); !ok {
		t.Fatalf("the swap was refused: %s", reason)
	}
	_, after := get(t, base+"/about")
	if !bytes.Contains(after, []byte("SWAPPED-TEXT")) {
		t.Fatalf("the page does not show the swapped code:\n%s", firstLines(after))
	}
	// A swap back to the first code gives the first page.
	if ok, reason := swap(t, base, pkg, "About_gx.go", src); !ok {
		t.Fatalf("the swap back was refused: %s", reason)
	}
	if _, again := get(t, base+"/about"); !bytes.Equal(again, before) {
		t.Fatal("the page after the swap back differs from the first page")
	}
}

// TestREQ_ACT_22_DevLogNamesTheBus checks that a dev build of an app with a
// shared signal writes the bus of its rooms to its log at startup, and that
// a production build writes no such line (REQ-ACT-22).
func TestREQ_ACT_22_DevLogNamesTheBus(t *testing.T) {
	shop := filepath.Join(repoRoot(t), "examples", "shop")
	logOf := func(tags ...string) string {
		bin := execname.Name(filepath.Join(t.TempDir(), "app"))
		args := append([]string{"build"}, tags...)
		build := exec.Command("go", append(args, "-o", bin, "./cmd/shop")...)
		build.Dir = shop
		if out, err := build.CombinedOutput(); err != nil {
			t.Fatalf("go build %v: %v\n%s", tags, err, out)
		}
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		addr := l.Addr().String()
		_ = l.Close()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		app := exec.CommandContext(ctx, bin, "-addr", addr)
		app.Dir = shop
		app.Env = append(os.Environ(), "GX_DEV_ADDR="+addr, "GX_DEV_SECRET="+devSecret)
		var log bytes.Buffer
		app.Stderr = &log
		if err := app.Start(); err != nil {
			t.Fatal(err)
		}
		// The app writes the line before it listens.
		for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
			if res, err := http.Get("http://" + addr + "/"); err == nil {
				_ = res.Body.Close()
				break
			}
		}
		cancel()
		_ = app.Wait()
		return log.String()
	}
	if dev := logOf("-tags", "gxdev"); !strings.Contains(dev, "gx: shared signals use the in-memory bus of Gx") {
		t.Errorf("the log of a dev build does not name the bus:\n%s", dev)
	}
	if prod := logOf(); strings.Contains(prod, "shared signals use") {
		t.Errorf("a production build names the bus in its log:\n%s", prod)
	}
}
