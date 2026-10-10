package tscheck_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
	"github.com/alternayte/gx/internal/tscheck"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// app writes an app with one island and its generated files.
func app(t *testing.T, island string) string {
	t.Helper()
	dir := writeTree(t, map[string]string{
		"go.mod":        "module app\n\ngo 1.25.0\n\nrequire github.com/alternayte/gx v0.0.0\n\nreplace github.com/alternayte/gx => " + filepath.ToSlash(repoRoot(t)) + "\n",
		"dash/props.go": "package dash\n\ntype ChartProps struct {\n\tTitle string `json:\"title\"`\n\tData  []int  `json:\"data\"`\n}\n",
		"dash/Chart.ts": island,
	})
	files, diags := compiler.Generate(dir)
	if len(diags) != 0 {
		t.Fatal(diags)
	}
	for path, src := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, src, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

const goodIsland = `import type { Mount } from "./Chart.props";

const mount: Mount = (el, { title, data }) => {
  el.textContent = title + ": " + data.reduce((a, b) => a + b, 0);
};

export default mount;
`

// The check runs the real pinned compiler, which is native code. PATH holds
// the Go tool only, so no node can take part.
func TestREQ_ISL_08_TypeErrorFailsTheCheckWithNoNode(t *testing.T) {
	bad := strings.Replace(goodIsland, "data.reduce((a, b) => a + b, 0)", "data.toUpperCase()", 1)
	dir := app(t, bad)
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", filepath.Dir(goTool))
	if _, err := exec.LookPath("node"); err == nil {
		t.Fatal("node is next to the go tool; the test cannot prove a check with no node")
	}
	diags, err := tscheck.App(context.Background(), dir, compiler.CheckOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(diags) != 1 {
		t.Fatalf("diagnostics = %v, want one", diags)
	}
	d := diags[0]
	if compiler.CodeIslandTypeScript != "GX6005" || d.Code != compiler.CodeIslandTypeScript {
		t.Fatalf("code = %s, want GX6005", d.Code)
	}
	if filepath.Base(d.File) != "Chart.ts" || !filepath.IsAbs(d.File) || d.Line != 4 || d.Col != 40 {
		t.Fatalf("position = %s:%d:%d, want Chart.ts:4:40", d.File, d.Line, d.Col)
	}
	if !strings.HasPrefix(d.Msg, "TS2339: ") || !strings.Contains(d.Msg, "toUpperCase") || !strings.Contains(d.Msg, "number[]") {
		t.Fatalf("message = %q", d.Msg)
	}

	if err := os.WriteFile(filepath.Join(dir, "dash", "Chart.ts"), []byte(goodIsland), 0o644); err != nil {
		t.Fatal(err)
	}
	diags, err = tscheck.App(context.Background(), dir, compiler.CheckOptions{})
	if err != nil || len(diags) != 0 {
		t.Fatalf("a correct island: %v, %v", diags, err)
	}
}

// The props type follows the Go struct: a new field is known to the island
// after gx generate, and a removed field is a TypeScript error.
func TestREQ_ISL_08_IslandSeesThePropsOfTheGoStruct(t *testing.T) {
	dir := app(t, strings.Replace(goodIsland, "{ title, data }", "{ title, data, missing }", 1))
	diags, err := (&tscheck.Manager{Root: dir}).Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(diags) != 1 || !strings.Contains(diags[0].Msg, "missing") || !strings.Contains(diags[0].Msg, "Props") {
		t.Fatalf("diagnostics = %v", diags)
	}
}

// The compiler is not asked while the generated files are stale: its
// errors would be about files that gx generate writes.
func TestREQ_ISL_08_StaleFilesComeFirst(t *testing.T) {
	dir := app(t, goodIsland)
	if err := os.Remove(filepath.Join(dir, "dash", "Chart.props.ts")); err != nil {
		t.Fatal(err)
	}
	diags, err := tscheck.App(context.Background(), dir, compiler.CheckOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(diags) != 1 || diags[0].Code != compiler.CodeStale {
		t.Fatalf("diagnostics = %v, want one GX1002", diags)
	}
}

// The islands of the example shop pass, with the pinned package d3-scale
// declared by js/vendor/pins.d.ts.
func TestREQ_ISL_08_ShopIslandsTypeCheck(t *testing.T) {
	shop := filepath.Join(repoRoot(t), "examples", "shop")
	diags, err := (&tscheck.Manager{Root: shop}).Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(diags) != 0 {
		t.Fatalf("the shop islands have TypeScript errors: %v", diags)
	}
}

// An app with a tsconfig.json is checked as that project.
func TestREQ_ISL_08_TsconfigIsTheProject(t *testing.T) {
	// The island assigns to an implicit any: an error only with strict.
	island := "export default (el: HTMLElement, props) => {\n  el.textContent = String(props);\n};\n"
	dir := app(t, island)
	diags, err := (&tscheck.Manager{Root: dir}).Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(diags) != 1 || !strings.HasPrefix(diags[0].Msg, "TS7006: ") {
		t.Fatalf("with the default options: %v", diags)
	}
	tsconfig := `{"compilerOptions": {"strict": false, "noEmit": true, "target": "es2022", "module": "esnext", "moduleResolution": "bundler", "lib": ["es2022", "dom"]}, "include": ["dash/*.ts"]}`
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(tsconfig), 0o644); err != nil {
		t.Fatal(err)
	}
	diags, err = (&tscheck.Manager{Root: dir}).Check(context.Background())
	if err != nil || len(diags) != 0 {
		t.Fatalf("with a tsconfig.json that turns strict off: %v, %v", diags, err)
	}
}

// pkg builds a platform package as the registry serves it.
func pkg(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func lockWith(t *testing.T, dir, version, asset, sum string) {
	t.Helper()
	data, _ := json.Marshal(map[string]any{"typescript": tscheck.LockEntry{Version: version, SHA256: map[string]string{asset: sum}}})
	if err := os.WriteFile(filepath.Join(dir, "gx.lock"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// The compiler is pinned by hash in gx.lock: a package with a different
// hash is refused, from the network and from .gx/vendor (SI-10).
func TestREQ_ISL_08_CompilerIsPinnedByHash(t *testing.T) {
	binary := "tsc"
	if runtime.GOOS == "windows" {
		binary = "tsc.exe"
	}
	body := pkg(t, map[string]string{
		"package/lib/" + binary:    "compiler",
		"package/lib/lib.d.ts":     "// lib",
		"package/lib/../../escape": "outside",
		"package/package.json":     "{}",
	})
	sum := sha256.Sum256(body)
	asset, err := tscheck.Asset("9.9.9", runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	dir := t.TempDir()
	lockWith(t, dir, "9.9.9", asset, strings.Repeat("0", 64))
	m := &tscheck.Manager{Root: dir, Cache: t.TempDir(), BaseURL: srv.URL}
	if _, err := m.Ensure(context.Background()); err == nil || !strings.Contains(err.Error(), "sha256") {
		t.Fatalf("with a wrong hash: err = %v", err)
	}

	lockWith(t, dir, "9.9.9", asset, hex.EncodeToString(sum[:]))
	bin, err := m.Ensure(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(bin); err != nil || string(got) != "compiler" || filepath.Base(bin) != binary {
		t.Fatalf("compiler file = %q, %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(bin), "lib.d.ts")); err != nil {
		t.Fatalf("the lib files are not next to the compiler: %v", err)
	}
	// No file of the package can leave the unpack directory.
	if matches, _ := filepath.Glob(filepath.Join(m.Cache, "*", "escape")); len(matches) != 0 {
		t.Fatalf("a path of the package left the directory: %v", matches)
	}
	if _, err := os.Stat(filepath.Join(m.Cache, "escape")); err == nil {
		t.Fatal("a path of the package left the directory")
	}
	platform := strings.TrimSuffix(strings.TrimPrefix(asset, "typescript-"), "-9.9.9.tgz")
	if want := "/@typescript/typescript-" + platform + "/-/" + asset; len(paths) != 2 || paths[1] != want {
		t.Fatalf("requests = %v, want %s", paths, want)
	}
	// The second call uses the cache.
	if _, err := m.Ensure(context.Background()); err != nil || len(paths) != 2 {
		t.Fatalf("second Ensure: %v, requests %v", err, paths)
	}

	// gx vendor stores the package; a changed vendored package is refused.
	vendored, err := m.Vendor(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(filepath.ToSlash(vendored), ".gx/vendor/typescript/9.9.9/") {
		t.Fatalf("vendored path = %s", vendored)
	}
	offline := &tscheck.Manager{Root: dir, Cache: t.TempDir(), BaseURL: "http://127.0.0.1:1"}
	if _, err := offline.Ensure(context.Background()); err != nil {
		t.Fatalf("with a vendored package and no network: %v", err)
	}
	if err := os.WriteFile(vendored, []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := offline.Ensure(context.Background()); err == nil || !strings.Contains(err.Error(), "gx.lock pins") {
		t.Fatalf("with a changed vendored package: err = %v", err)
	}
}

// An app with no island needs no compiler and no network.
func TestREQ_ISL_08_NoIslandNoDownload(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":       "module app\n\ngo 1.25.0\n",
		"home/Home.gx": "package home\n\n<main>Home</main>\n",
	})
	m := &tscheck.Manager{Root: dir, Cache: t.TempDir(), BaseURL: "http://127.0.0.1:1"}
	diags, err := m.Check(context.Background())
	if err != nil || len(diags) != 0 {
		t.Fatalf("Check = %v, %v", diags, err)
	}
}

func TestREQ_ISL_08_BuiltInPinCoversEachPlatform(t *testing.T) {
	lock := tscheck.DefaultLock()
	for _, target := range [][2]string{{"darwin", "arm64"}, {"darwin", "amd64"}, {"linux", "amd64"}, {"linux", "arm64"}, {"windows", "amd64"}, {"windows", "arm64"}} {
		asset, err := tscheck.Asset(lock.Version, target[0], target[1])
		if err != nil {
			t.Fatal(err)
		}
		if len(lock.SHA256[asset]) != 64 {
			t.Errorf("no sha256 for %s", asset)
		}
	}
	if _, err := tscheck.Asset(lock.Version, "plan9", "386"); err == nil {
		t.Error("plan9/386 has a compiler")
	}
}

// TestREQ_ACT_20_ClientTypeChecks checks that the pinned TypeScript compiler
// reads the client of gx api with the islands: the client of the shop has no
// error, and an error in a client fails the check (REQ-ACT-20).
func TestREQ_ACT_20_ClientTypeChecks(t *testing.T) {
	shop := filepath.Join(repoRoot(t), "examples", "shop")
	src, err := os.ReadFile(filepath.Join(shop, compiler.APIDir, "client.ts"))
	if err != nil {
		t.Fatal(err)
	}
	// The client alone, in a module with no island.
	dir := t.TempDir()
	path := filepath.Join(dir, compiler.APIDir, "client.ts")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, src, 0o644); err != nil {
		t.Fatal(err)
	}
	diags, err := (&tscheck.Manager{Root: dir}).Check(context.Background())
	if err != nil || len(diags) != 0 {
		t.Fatalf("the client of the shop: %v, %v", diags, err)
	}
	// A use of the client with a field of the wrong type is an error.
	use := "\nexport const wrong = createClient().basketSetQty({ sku: 'milk', qty: 'two' })\n"
	if err := os.WriteFile(path, append(src, use...), 0o644); err != nil {
		t.Fatal(err)
	}
	diags, err = (&tscheck.Manager{Root: dir}).Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(diags) != 1 || diags[0].Code != compiler.CodeIslandTypeScript || !strings.Contains(diags[0].Msg, "string") {
		t.Fatalf("a wrong argument type: %v, want one TypeScript error", diags)
	}
}
