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

// startDevApp builds the app of dir with the gxdev tag and runs it.
func startDevApp(t *testing.T, dir, mainPkg string) string {
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
	app.Env = append(os.Environ(), "GX_DEV_ADDR="+addr)
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
	res, err := http.Post(base+"/_gx/dev/swap", "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
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
	// The generated code has the page text as string literals. This is
	// what a markup edit changes.
	i := bytes.Index(src, []byte(`gx.Text("`))
	if i < 0 {
		t.Fatal("About_gx.go has no text node")
	}
	edited := append(append(append([]byte{}, src[:i]...), []byte(`gx.Text("SWAPPED-TEXT" + strconv.Itoa(len(p.GxNoSuchField)) + "`)...), src[i+len(`gx.Text("`):]...)
	ok, reason := swap(t, base, pkg, "About_gx.go", edited)
	if ok || !strings.Contains(reason, "the symbol table has no name strconv") {
		t.Fatalf("a swap that uses a package outside the imports of the file = %v, %q", ok, reason)
	}
	if _, after := get(t, base+"/about"); !bytes.Equal(after, before) {
		t.Fatal("a refused swap changed the page")
	}

	edited = append(append(append([]byte{}, src[:i]...), []byte(`gx.Text("SWAPPED-TEXT`)...), src[i+len(`gx.Text("`):]...)
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
