package gxcli_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
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

	"github.com/alternayte/gx/gxcli"
	"github.com/alternayte/gx/internal/execname"
	"github.com/alternayte/gx/internal/pagefind"
	"github.com/alternayte/gx/internal/tailwind"
)

// fakePagefindTarball builds a release tarball that holds one script.
func fakePagefindTarball(t *testing.T) []byte {
	t.Helper()
	name := "pagefind"
	if runtime.GOOS == "windows" {
		name = "pagefind.exe"
	}
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	body := []byte("#!/bin/sh\necho pagefind\n")
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// packJSON is a three-icon Iconify pack.
const packJSON = `{
  "prefix": "lucide",
  "width": 24,
  "height": 24,
  "icons": {
    "shopping-cart": {"body": "<circle cx=\"8\" cy=\"21\" r=\"1\"/><path d=\"M1 1\"/>"},
    "arrow-right": {"body": "<path d=\"M5 12h14\"/>"}
  },
  "aliases": {
    "cart": {"parent": "shopping-cart"}
  }
}`

// TestREQ_STY_06_IconsPin covers `gx icons pin`: the pack is fetched,
// pinned by sha256 in gx.lock, and one .gx component is written per icon
// with a tamper check on the next pin (REQ-STY-06).
func TestREQ_STY_06_IconsPin(t *testing.T) {
	body := packJSON
	sum := sha256.Sum256([]byte(body))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/lucide.json" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	t.Setenv("GX_ICONIFY_BASE", srv.URL)

	dir := t.TempDir()
	if code := gxcli.Main([]string{"icons", "pin", "lucide@1.2.3", dir}); code != 0 {
		t.Fatalf("icons pin exit = %d", code)
	}
	for name, want := range map[string]string{
		"ShoppingCart.gx": "gx.Icon(",
		"Cart.gx":         "<circle cx=",
		"ArrowRight.gx":   "M5 12h14",
	} {
		data, err := os.ReadFile(filepath.Join(dir, "ui", "icons", "lucide", name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		got := string(data)
		for _, needle := range []string{"package lucide", "props {", "Label string", "Class string", want} {
			if !strings.Contains(got, needle) {
				t.Fatalf("%s lacks %q:\n%s", name, needle, got)
			}
		}
	}
	lock, err := os.ReadFile(filepath.Join(dir, "gx.lock"))
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Icons map[string]struct {
			Version string `json:"version"`
			SHA256  string `json:"sha256"`
		} `json:"icons"`
	}
	if err := json.Unmarshal(lock, &parsed); err != nil {
		t.Fatal(err)
	}
	pin, ok := parsed.Icons["lucide"]
	if !ok || pin.Version != "1.2.3" || pin.SHA256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("gx.lock icons = %+v", parsed.Icons)
	}

	// A changed pack with the same version stops the next pin.
	tampered := strings.Replace(body, "M1 1", "M2 2", 1)
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(tampered))
	}))
	defer srv2.Close()
	t.Setenv("GX_ICONIFY_BASE", srv2.URL)
	if code := gxcli.Main([]string{"icons", "pin", "lucide@1.2.3", dir}); code == 0 {
		t.Fatal("icons pin accepted a changed pack for a pinned version")
	}
}

// TestREQ_STY_06_IconDeadCode covers the linker dead-code budget: only the
// icon that the app uses is in the binary (REQ-STY-06).
func TestREQ_STY_06_IconDeadCode(t *testing.T) {
	body := packJSON
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	t.Setenv("GX_ICONIFY_BASE", srv.URL)

	repo, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	mod := "module app\n\ngo 1.25.0\n\nrequire github.com/alternayte/gx v0.0.0\n\nreplace github.com/alternayte/gx => " + filepath.ToSlash(repo) + "\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(mod), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := gxcli.Main([]string{"icons", "pin", "lucide@1.2.3", dir}); code != 0 {
		t.Fatalf("icons pin exit = %d", code)
	}
	main := `package main

import (
	"fmt"
	"os"

	"app/ui/icons/lucide"

	"github.com/alternayte/gx"
)

func main() {
	fmt.Fprint(os.Stdout, gx.String(lucide.ShoppingCart(lucide.ShoppingCartProps{Class: "size-4"})))
}
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(main), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := gxcli.Main([]string{"generate", dir}); code != 0 {
		t.Fatalf("generate exit = %d", code)
	}
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out)
		}
	}
	run("go", "mod", "tidy")
	bin := execname.Name(filepath.Join(dir, "app"))
	run("go", "build", "-o", bin, ".")
	data, err := os.ReadFile(bin)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "M1 1") {
		t.Fatal("the used icon is not in the binary")
	}
	if strings.Contains(string(data), "M5 12h14") {
		t.Fatal("an unused icon is in the binary; the linker did not drop it")
	}
}

// TestREQ_STY_12_VendorCommand covers `gx vendor`: the pinned Tailwind
// binary lands in .gx/vendor for offline builds (REQ-STY-12).
func TestREQ_STY_12_VendorCommand(t *testing.T) {
	content := []byte("#!/bin/sh\necho hi\n")
	sum := sha256.Sum256(content)
	hexSum := hex.EncodeToString(sum[:])
	asset, err := tailwind.Asset(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	pfAsset, err := pagefind.Asset(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	pfData := fakePagefindTarball(t)
	pfSum := sha256.Sum256(pfData)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, asset):
			_, _ = w.Write(content)
		case strings.HasSuffix(r.URL.Path, pfAsset):
			_, _ = w.Write(pfData)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	lock := map[string]any{
		"tailwind": map[string]any{
			"version": "v1.2.3",
			"sha256":  map[string]string{asset: hexSum},
		},
		"pagefind": map[string]any{
			"version": pagefind.DefaultVersion,
			"sha256":  map[string]string{pfAsset: hex.EncodeToString(pfSum[:])},
		},
	}
	data, err := json.Marshal(lock)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "gx.lock"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "gx.toml"), []byte("[mirrors]\ntailwind = \""+srv.URL+"\"\npagefind = \""+srv.URL+"\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := gxcli.Main([]string{"vendor", dir}); code != 0 {
		t.Fatalf("vendor exit = %d", code)
	}
	if _, err := os.Stat(filepath.Join(dir, ".gx", "vendor", "tailwind", "v1.2.3", asset)); err != nil {
		t.Fatalf("vendored tailwind: %v", err)
	}
	pfName := "pagefind"
	if runtime.GOOS == "windows" {
		pfName = "pagefind.exe"
	}
	if _, err := os.Stat(filepath.Join(dir, ".gx", "vendor", "pagefind", pagefind.DefaultVersion, pfName)); err != nil {
		t.Fatalf("vendored pagefind: %v", err)
	}
}
