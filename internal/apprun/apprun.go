// Package apprun builds an app with the gxdev tag and runs it on a free
// local port. The static export and the dev MCP server read the running
// app through it (REQ-EXP-01, REQ-AI-04).
package apprun

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/alternayte/gx/internal/compiler"
	"github.com/alternayte/gx/internal/execname"
	"github.com/alternayte/gx/internal/gxstyles"
	"github.com/alternayte/gx/internal/islands"
)

// App is one running dev build of an app.
type App struct {
	// Base is the address of the app, for example http://127.0.0.1:53211.
	Base string
	cmd  *exec.Cmd
	work string
}

// Start generates the code of the app in dir, builds its main package with
// the gxdev tag and starts it. An empty mainPkg is found with DetectMain.
// The caller stops the app.
func Start(ctx context.Context, dir, mainPkg string) (*App, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if mainPkg == "" {
		mainPkg, err = DetectMain(dir)
		if err != nil {
			return nil, err
		}
	}
	work, err := os.MkdirTemp("", "gx-app-")
	if err != nil {
		return nil, err
	}
	bin := execname.Name(filepath.Join(work, "app"))
	if err := build(ctx, dir, mainPkg, bin); err != nil {
		_ = os.RemoveAll(work)
		return nil, err
	}
	port, err := freePort()
	if err != nil {
		_ = os.RemoveAll(work)
		return nil, err
	}
	cmd := exec.CommandContext(ctx, bin)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GX_DEV_ADDR=127.0.0.1:"+strconv.Itoa(port), "GX_DEV=1")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		_ = os.RemoveAll(work)
		return nil, err
	}
	app := &App{Base: "http://127.0.0.1:" + strconv.Itoa(port), cmd: cmd, work: work}
	if err := waitReady(ctx, port); err != nil {
		app.Stop()
		return nil, err
	}
	return app, nil
}

// Stop ends the app and removes its binary.
func (a *App) Stop() {
	if a == nil {
		return
	}
	if a.cmd != nil && a.cmd.Process != nil {
		_ = a.cmd.Process.Kill()
		_, _ = a.cmd.Process.Wait()
	}
	if a.work != "" {
		_ = os.RemoveAll(a.work)
	}
}

// build generates the app code and the stylesheet, then builds the app with
// the gxdev tag.
func build(ctx context.Context, dir, mainPkg, bin string) error {
	files, diags := compiler.NewSession().Generate(dir)
	if len(diags) > 0 {
		return fmt.Errorf("%s", diags[0].String())
	}
	for path, src := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, src, 0o644); err != nil {
			return err
		}
	}
	if _, err := gxstyles.Build(ctx, dir, true); err != nil {
		return err
	}
	if _, err := islands.Write(dir, islands.Options{Minify: true}); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "go", "build", "-tags", "gxdev", "-o", bin, mainPkg)
	cmd.Dir = dir
	// A fresh module may need to record the gx dependency graph.
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("build: %w\n%s", err, out)
	}
	return nil
}

// DetectMain finds the app main package: ./cmd/app, or the only cmd/*
// directory with a main.go. cmd/gx is the project's own Gx command line
// tool, not the app.
func DetectMain(dir string) (string, error) {
	if _, err := os.Stat(filepath.Join(dir, "cmd", "app", "main.go")); err == nil {
		return "./cmd/app", nil
	}
	entries, err := os.ReadDir(filepath.Join(dir, "cmd"))
	if err != nil {
		return "", fmt.Errorf("no -main given and no cmd/ directory found")
	}
	var found []string
	for _, e := range entries {
		if !e.IsDir() || e.Name() == "gx" {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, "cmd", e.Name(), "main.go")); err == nil {
			found = append(found, "./cmd/"+e.Name())
		}
	}
	switch len(found) {
	case 0:
		return "", fmt.Errorf("no -main given and no cmd/*/main.go found")
	case 1:
		return found[0], nil
	default:
		return "", fmt.Errorf("several mains found (%s); pass -main", strings.Join(found, ", "))
	}
}

// waitReady blocks until the app accepts a connection.
func waitReady(ctx context.Context, port int) error {
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", "127.0.0.1:"+strconv.Itoa(port), 200*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
	return fmt.Errorf("the app did not start")
}

// freePort returns a free local port.
func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}
