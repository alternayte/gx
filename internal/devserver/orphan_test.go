//go:build !windows

// The test sends unix signals.

package devserver_test

import (
	"bufio"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"syscall"
	"testing"
	"time"

	"github.com/alternayte/gx/internal/execname"
)

// devLine is the line gx dev prints when the proxy and the app are up.
var devLine = regexp.MustCompile(`gx dev: http://\S+ \(app http://(127\.0\.0\.1:\d+)\)`)

// TestREQ_DEV_01_AppStopsWithDevServer covers the end of a dev session: the
// app process stops when gx dev stops, also when gx dev gets no chance to
// clean up. A closed terminal sends SIGHUP; SIGKILL cannot be caught. An app
// that outlives gx dev keeps its port and its memory until someone finds it.
func TestREQ_DEV_01_AppStopsWithDevServer(t *testing.T) {
	root := repoRoot(t)
	gxBin := execname.Name(filepath.Join(t.TempDir(), "gx"))
	build := exec.Command("go", "build", "-o", gxBin, "./cmd/gx")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build gx: %v\n%s", err, out)
	}
	for _, sig := range []syscall.Signal{syscall.SIGHUP, syscall.SIGKILL} {
		t.Run(sig.String(), func(t *testing.T) {
			dir := t.TempDir()
			mod := "module app\n\ngo 1.25.0\n\nrequire github.com/alternayte/gx v0.0.0\n\nreplace github.com/alternayte/gx => " +
				filepath.ToSlash(root) + "\n"
			if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(mod), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Join(dir, "cmd", "app"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "cmd", "app", "main.go"), []byte(gxAppMain), 0o644); err != nil {
				t.Fatal(err)
			}
			dev := exec.Command(gxBin, "dev", "-addr", freeAddr(t), "-main", "./cmd/app", dir)
			out, err := dev.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			dev.Stderr = os.Stderr
			if err := dev.Start(); err != nil {
				t.Fatal(err)
			}
			// A leftover app is found by the temp directory in its path.
			t.Cleanup(func() {
				_ = dev.Process.Kill()
				_ = exec.Command("pkill", "-f", dir).Run()
			})
			found := make(chan string, 1)
			go func() {
				sc := bufio.NewScanner(out)
				for sc.Scan() {
					if m := devLine.FindStringSubmatch(sc.Text()); m != nil {
						found <- m[1]
						return
					}
				}
				close(found)
			}()
			var app string
			select {
			case app = <-found:
			case <-time.After(90 * time.Second):
				t.Fatal("gx dev did not start")
			}
			if app == "" {
				t.Fatal("gx dev ended before it started the app")
			}
			if conn, err := net.DialTimeout("tcp", app, 2*time.Second); err != nil {
				t.Fatalf("the app does not listen on %s: %v", app, err)
			} else {
				_ = conn.Close()
			}
			if err := dev.Process.Signal(sig); err != nil {
				t.Fatal(err)
			}
			_ = dev.Wait()
			deadline := time.Now().Add(5 * time.Second)
			for {
				conn, err := net.DialTimeout("tcp", app, 500*time.Millisecond)
				if err != nil {
					return
				}
				_ = conn.Close()
				if time.Now().After(deadline) {
					t.Fatalf("the app still listens on %s after gx dev got %s", app, sig)
				}
				time.Sleep(100 * time.Millisecond)
			}
		})
	}
}
