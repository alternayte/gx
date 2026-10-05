package gxcli_test

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/alternayte/gx/internal/execname"
)

// TestNFR_07_NoNodeWorkflows covers the default user workflows with no node
// tool on the PATH: a new app, format, check, generate, new code, a
// registry component, routes, describe, lint, a production build with the
// real Tailwind binary, the dev server, and a docs site exported with the
// real Pagefind binary (NFR-07, G3).
func TestNFR_07_NoNodeWorkflows(t *testing.T) {
	repo := thisRepo(t)
	gx := execname.Name(filepath.Join(t.TempDir(), "gx"))
	build := exec.Command("go", "build", "-o", gx, "./cmd/gx")
	build.Dir = repo
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build gx: %v\n%s", err, out)
	}
	env := append(withoutNode(t, os.Environ()), "GOFLAGS=-mod=mod")
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	registry := filepath.Join(repo, "registry")

	run := func(dir string, args ...string) string {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, gx, args...)
		cmd.Dir = dir
		cmd.Env = env
		cmd.Stdin = strings.NewReader("")
		var out bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &out
		if err := cmd.Run(); err != nil {
			t.Fatalf("gx %s: %v\n%s", strings.Join(args, " "), err, out.String())
		}
		return out.String()
	}
	freeAddr := func() string {
		t.Helper()
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer l.Close()
		return l.Addr().String()
	}
	get := func(url, want string) string {
		t.Helper()
		deadline := time.Now().Add(90 * time.Second)
		for {
			resp, err := http.Get(url)
			if err == nil {
				body, _ := io.ReadAll(resp.Body)
				_ = resp.Body.Close()
				if resp.StatusCode == http.StatusOK && strings.Contains(string(body), want) {
					return string(body)
				}
				if time.Now().After(deadline) {
					t.Fatalf("GET %s = %d without %q:\n%s", url, resp.StatusCode, want, body)
				}
			} else if time.Now().After(deadline) {
				t.Fatalf("GET %s: %v", url, err)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}

	// A new app, with no answer on stdin: the adapter question takes its
	// default.
	app := filepath.Join(parent, "nonode")
	run(parent, "init", "--module", "example.com/nonode", "--replace", repo, "--registry", registry, app)
	for _, file := range []string{"home/Home.gx", "home/Counter.gx", "app/Shell.gx"} {
		run(app, "fmt", "--check", file)
	}
	run(app, "check")
	run(app, "new", "slice", "shop")
	run(app, "new", "page", "shop/Detail")
	run(app, "new", "component", "ui/tag/Tag")
	run(app, "add", "button")
	run(app, "generate")
	run(app, "check")
	if out := run(app, "routes"); !strings.Contains(out, "GET /shop/detail") {
		t.Fatalf("gx routes:\n%s", out)
	}
	if out := run(app, "describe", "--json"); !strings.Contains(out, `"module": "example.com/nonode"`) {
		t.Fatalf("gx describe:\n%s", out)
	}
	run(app, "lint")

	// A production build runs the pinned Tailwind binary.
	run(app, "build")
	bin := execname.Name(filepath.Join(app, "bin", "app"))
	addr := freeAddr()
	server := exec.Command(bin)
	server.Dir = t.TempDir()
	server.Env = append(env, "GX_DEV_ADDR="+addr)
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = server.Process.Kill()
		_, _ = server.Process.Wait()
	}()
	get("http://"+addr+"/", "Welcome to nonode")
	css := get("http://"+addr+"/_gx/app.css", "--background")
	for _, class := range []string{".rounded-xl", ".text-muted-foreground"} {
		if !strings.Contains(css, class) {
			t.Fatalf("the built stylesheet lacks %s, a class of the scaffold", class)
		}
	}

	// The dev server: one command, the app behind the proxy with the dev
	// client.
	devAddr := freeAddr()
	dev := exec.Command(gx, "dev", "--addr", devAddr)
	dev.Dir = app
	dev.Env = env
	var devOut bytes.Buffer
	dev.Stdout, dev.Stderr = &devOut, &devOut
	if err := dev.Start(); err != nil {
		t.Fatal(err)
	}
	devDone := make(chan struct{})
	go func() { _ = dev.Wait(); close(devDone) }()
	defer func() {
		select {
		case <-devDone:
		default:
			_ = dev.Process.Kill()
			<-devDone
		}
	}()
	page := get("http://"+devAddr+"/", "/_gx/dev-client.js")
	if !strings.Contains(page, "Welcome to nonode") {
		t.Fatalf("gx dev serves:\n%s\n%s", page, devOut.String())
	}
	if err := dev.Process.Signal(syscall.SIGTERM); err != nil {
		_ = dev.Process.Kill()
	}
	select {
	case <-devDone:
	case <-time.After(30 * time.Second):
		t.Fatalf("gx dev did not stop:\n%s", devOut.String())
	}

	// A docs site: the registry items, the check, and a static export with
	// the pinned Pagefind binary.
	docs := filepath.Join(parent, "nonodedocs")
	run(parent, "init", "--template", "docs", "--module", "example.com/nonodedocs", "--replace", repo, "--registry", registry, docs)
	run(docs, "check")
	dist := filepath.Join(docs, "dist")
	run(docs, "export", "--out", dist)
	for _, rel := range []string{"index.html", "start/index.html", "404.html", "llms.txt", "pagefind/pagefind.js"} {
		if _, err := os.Stat(filepath.Join(dist, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("the export lacks %s", rel)
		}
	}
	index, err := os.ReadFile(filepath.Join(dist, "start", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(index, []byte("/_gx/app.")) || !bytes.Contains(index, []byte("Run the site")) {
		t.Fatalf("the exported page has no hashed stylesheet or no content:\n%s", index)
	}
}
