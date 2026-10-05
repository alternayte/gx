package gxcli_test

import (
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alternayte/gx/gxcli"
	"github.com/alternayte/gx/internal/execname"
	"github.com/alternayte/gx/internal/gxstyles"
)

// TestNFR_08_OneBinary covers the production build: `gx build` writes one
// binary, and that binary serves its pages, the Gx runtime, the adapter
// runtime, the stylesheet and the app's public files with no file of the
// app beside it (NFR-08).
func TestNFR_08_OneBinary(t *testing.T) {
	t.Setenv("GOFLAGS", "-mod=mod")
	dir := scratchModule(t, map[string]string{
		"home/route/route.go":   "package route\n\nimport \"github.com/alternayte/gx\"\n\ntype Home struct {\n\tgx.Route `GET /{$}`\n}\n",
		"home/page.go":          "package home\n\nimport (\n\t\"github.com/alternayte/gx\"\n\t\"app/home/route\"\n)\n\nvar Page = gx.Page(func(c *gx.Ctx, in route.Home) (ViewProps, error) {\n\treturn ViewProps{}, nil\n}, View)\n\nvar Routes = gx.Collect(Page)\n",
		"home/View.gx":          "package home\n\nsignals {\n  Open bool = false\n}\n\n<main>\n  <img src=\"/logo.svg\" alt=\"Logo\" />\n  <button on:click={$Open = !$Open}>Menu</button>\n  <p show={$Open}>One binary</p>\n</main>\n",
		"public/logo.svg":       "<svg xmlns=\"http://www.w3.org/2000/svg\"><title>embedded logo</title></svg>\n",
		"gxstyles/styles_gx.go": string(gxstyles.Generate([]byte(".embedded-rule{color:red}"))),
		"cmd/app/main.go":       "package main\n\nimport (\n\t\"net/http\"\n\t\"os\"\n\n\t\"github.com/alternayte/gx\"\n\t\"github.com/alternayte/gx/adapters/datastar\"\n\t\"app\"\n\t\"app/gxstyles\"\n\t\"app/home\"\n)\n\nfunc main() {\n\tgx.SetStylesheet(gxstyles.CSS())\n\tapp := gx.New(gx.Config{Adapter: datastar.Adapter(), Public: assets.Public()})\n\tapp.Group(\"/\", home.Routes)\n\t_ = http.ListenAndServe(os.Getenv(\"GX_DEV_ADDR\"), app)\n}\n",
		"assets.go":             "// Package assets embeds the public files of the app.\npackage assets\n\nimport (\n\t\"embed\"\n\t\"io/fs\"\n)\n\n//go:embed public\nvar files embed.FS\n\n// Public returns the embedded public directory.\nfunc Public() fs.FS {\n\tsub, err := fs.Sub(files, \"public\")\n\tif err != nil {\n\t\tpanic(err)\n\t}\n\treturn sub\n}\n",
	})

	away := t.TempDir()
	bin := execname.Name(filepath.Join(away, "app"))
	if code := gxcli.Main([]string{"build", "-o", bin, dir}); code != 0 {
		t.Fatalf("gx build exit = %d", code)
	}
	entries, err := os.ReadDir(away)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("gx build wrote %d files, want one binary", len(entries))
	}
	// The binary must not read the app tree: remove it before the run.
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	cmd := exec.Command(bin)
	cmd.Dir = away
	cmd.Env = append(os.Environ(), "GX_DEV_ADDR="+addr)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()

	get := func(path string) (int, string) {
		t.Helper()
		deadline := time.Now().Add(20 * time.Second)
		for {
			resp, err := http.Get("http://" + addr + path)
			if err == nil {
				defer resp.Body.Close()
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					t.Fatal(err)
				}
				return resp.StatusCode, string(body)
			}
			if time.Now().After(deadline) {
				t.Fatalf("GET %s: %v", path, err)
			}
			time.Sleep(50 * time.Millisecond)
		}
	}

	status, page := get("/")
	if status != http.StatusOK || !strings.Contains(page, "One binary") {
		t.Fatalf("GET / = %d\n%s", status, page)
	}
	for _, want := range []string{"/_gx/gx.js", "/_gx/datastar.js", "/_gx/app.css"} {
		if !strings.Contains(page, want) {
			t.Fatalf("the page does not load %s:\n%s", want, page)
		}
	}
	for path, want := range map[string]string{
		"/_gx/gx.js":       "",
		"/_gx/datastar.js": "",
		"/_gx/behavior.js": "",
		"/_gx/tabs.js":     "",
		"/_gx/toast.js":    "",
		"/_gx/overlay.js":  "",
		"/_gx/theme.js":    "",
		"/_gx/app.css":     ".embedded-rule{color:red}",
		"/logo.svg":        "embedded logo",
	} {
		status, body := get(path)
		if status != http.StatusOK || body == "" || !strings.Contains(body, want) {
			t.Fatalf("GET %s = %d, %d bytes", path, status, len(body))
		}
	}
	if status, _ := get("/missing.svg"); status != http.StatusNotFound {
		t.Fatalf("GET /missing.svg = %d, want 404", status)
	}
}
