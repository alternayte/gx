package docscheck_test

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alternayte/gx/internal/compiler"
	"github.com/alternayte/gx/internal/docscheck"
	"github.com/alternayte/gx/internal/execname"
	"github.com/alternayte/gx/internal/exporter"
	"github.com/alternayte/gx/internal/islands"
	"github.com/alternayte/gx/internal/registry"
	"github.com/alternayte/gx/internal/scaffold"
)

// The samples of the docs build on the app that `gx init acme` writes. One
// scaffold is made and copied for each sample app.
var base struct {
	once sync.Once
	dir  string
	err  error
}

func baseApp(repo string) (string, error) {
	base.once.Do(func() {
		parent, err := os.MkdirTemp("", "gx-docs-samples-")
		if err != nil {
			base.err = err
			return
		}
		base.dir = filepath.Join(parent, "acme")
		_, base.err = scaffold.Init(scaffold.Options{
			Dir: base.dir, Adapter: "datastar", Version: "0.1.0", Replace: repo,
			Registry: filepath.Join(repo, "registry"),
		})
	})
	return base.dir, base.err
}

func TestMain(m *testing.M) {
	// The sample apps resolve the gx module through a replace.
	os.Setenv("GOFLAGS", "-mod=mod")
	code := m.Run()
	if base.dir != "" {
		_ = os.RemoveAll(filepath.Dir(base.dir))
	}
	os.Exit(code)
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

// gxCommands is the set of commands that `gx help` lists.
func gxCommands(t *testing.T, repo string) map[string]bool {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repo, "gxcli", "gxcli.go"))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{"help": true}
	in := false
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "Commands:") {
			in = true
			continue
		}
		if in {
			if strings.HasPrefix(line, "`)") {
				break
			}
			if fields := strings.Fields(line); len(fields) > 0 && strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "   ") {
				out[fields[0]] = true
			}
		}
	}
	if len(out) < 15 {
		t.Fatalf("found %d gx commands in the usage text", len(out))
	}
	return out
}

// samplePages returns the hand-written pages: the component pages are
// generated and the diagnostic pages have their own test.
func samplePages(t *testing.T, repo string) []docscheck.Page {
	t.Helper()
	all, err := docscheck.Pages(docscheck.ContentDir(repo))
	if err != nil {
		t.Fatal(err)
	}
	var out []docscheck.Page
	for _, p := range all {
		if strings.HasPrefix(p.Slug, "components/") || p.Slug == "components" || strings.HasPrefix(p.Slug, "errors/") {
			continue
		}
		// The API reference holds the signatures of the gx source, which
		// the compiler checks.
		if p.Meta["generated"] == "api" {
			continue
		}
		out = append(out, p)
	}
	// The README shows the same kind of sample as a docs page.
	readme, err := docscheck.ReadPage(filepath.Join(repo, "README.md"), "README")
	if err != nil {
		t.Fatal(err)
	}
	return append(out, readme)
}

// TestREQ_DOC_04_Samples covers every code sample of the hand-written docs
// pages. A gx or go block names its file, so it is a whole file. The test
// starts from the app of `gx init acme`, runs the `gx new` and `gx add`
// commands of the page, writes the files, and then the app must pass the
// check and build. A block titled "GET /path" is what the reader sees: the
// test runs the app and finds each line in the answer. The pages of one
// tutorial build one app, in order (REQ-DOC-04).
func TestREQ_DOC_04_Samples(t *testing.T) {
	repo := repoRoot(t)
	commands := gxCommands(t, repo)
	apps := map[string][]docscheck.Page{}
	var names []string
	for _, p := range samplePages(t, repo) {
		for _, b := range p.Blocks() {
			if (b.Lang == "gx" || b.Lang == "go") && b.Title == "" {
				t.Errorf("%s: the %s block at line %d names no file; write title=\"path\" so the gate compiles it", p.Slug, b.Lang, b.Line)
			}
			if b.Lang == "sh" {
				for _, args := range gxLines(b.Code) {
					if !commands[args[0]] {
						t.Errorf("%s: line %d runs gx %s, which is not a gx command", p.Slug, b.Line, args[0])
					}
				}
			}
		}
		name := p.Meta["sample"]
		if name == "" {
			if !hasSample(p) {
				continue
			}
			name = "page:" + p.Slug
		}
		if _, ok := apps[name]; !ok {
			names = append(names, name)
		}
		apps[name] = append(apps[name], p)
	}
	if len(names) < 8 {
		t.Errorf("the docs hold %d sample apps; the tutorial and the guides give more", len(names))
	}
	for _, name := range names {
		pages := apps[name]
		sort.SliceStable(pages, func(i, j int) bool {
			a, _ := strconv.Atoi(pages[i].Meta["order"])
			b, _ := strconv.Atoi(pages[j].Meta["order"])
			return a < b
		})
		t.Run(strings.NewReplacer("/", "_", ":", "_").Replace(name), func(t *testing.T) {
			t.Parallel()
			template, err := baseApp(repo)
			if err != nil {
				t.Fatalf("gx init: %v", err)
			}
			// Each app builds and links a binary; too many at one time
			// make each one slow.
			sampleSlots <- struct{}{}
			defer func() { <-sampleSlots }()
			dir := filepath.Join(t.TempDir(), "acme")
			if err := copyTree(template, dir); err != nil {
				t.Fatal(err)
			}
			// The samples do not run the project CLI. Without it, a build
			// of the app does not link the whole gx command.
			if err := os.RemoveAll(filepath.Join(dir, "cmd", "gx")); err != nil {
				t.Fatal(err)
			}
			for _, p := range pages {
				applyPage(t, repo, dir, p)
			}
		})
	}
}

// sampleSlots limits the sample apps that build at one time.
var sampleSlots = make(chan struct{}, 4)

// hasSample reports whether a page writes a file or expects an answer.
func hasSample(p docscheck.Page) bool {
	for _, b := range p.Blocks() {
		if b.Title != "" && (isFileBlock(b) || strings.HasPrefix(b.Title, "GET ")) {
			return true
		}
	}
	return false
}

// isFileBlock reports whether a block is a file of the sample app.
func isFileBlock(b docscheck.Block) bool {
	if b.Title == "" || strings.Contains(b.Title, " ") || b.Lang == "sh" {
		return false
	}
	return strings.Contains(b.Title, ".") || strings.Contains(b.Title, "/")
}

// gxLines returns the arguments of each gx command line of a shell block.
func gxLines(code string) [][]string {
	var out [][]string
	for _, line := range strings.Split(code, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "$ "))
		for _, prefix := range []string{"go run ./cmd/gx ", "gx "} {
			if rest, ok := strings.CutPrefix(line, prefix); ok {
				if fields := strings.Fields(rest); len(fields) > 0 {
					out = append(out, fields)
				}
				break
			}
		}
	}
	return out
}

// applyPage runs the commands and writes the files of one page, then
// checks, builds and runs the app.
func applyPage(t *testing.T, repo, dir string, p docscheck.Page) {
	t.Helper()
	var gets []docscheck.Block
	exportReport := ""
	for _, b := range p.Blocks() {
		switch {
		case b.Lang == "text" && b.Title == "gx export":
			exportReport = strings.TrimRight(b.Code, "\n")
		case b.Lang == "sh":
			for _, args := range gxLines(b.Code) {
				switch args[0] {
				case "new":
					if len(args) != 3 {
						t.Fatalf("%s: gx %s: want a kind and a name", p.Slug, strings.Join(args, " "))
					}
					if _, err := scaffold.New(dir, args[1], args[2]); err != nil {
						t.Fatalf("%s: gx %s: %v", p.Slug, strings.Join(args, " "), err)
					}
				case "add":
					installer := registry.Installer{Root: dir, Source: filepath.Join(repo, "registry"), Dir: "ui"}
					for _, item := range args[1:] {
						if _, err := installer.Add(item, ""); err != nil {
							t.Fatalf("%s: gx add %s: %v", p.Slug, item, err)
						}
					}
				}
			}
		case strings.HasPrefix(b.Title, "GET "):
			gets = append(gets, b)
		case isFileBlock(b):
			if filepath.IsAbs(b.Title) || strings.Contains(b.Title, "..") {
				t.Fatalf("%s: the file %q is not inside the app", p.Slug, b.Title)
			}
			path := filepath.Join(dir, filepath.FromSlash(b.Title))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(b.Code), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := scaffold.Generate(dir); err != nil {
		t.Fatalf("%s: the samples do not generate: %v", p.Slug, err)
	}
	// gx build bundles the islands of the app (REQ-ISL-03).
	if _, err := islands.Write(dir, islands.Options{Minify: true}); err != nil {
		t.Fatalf("%s: the islands of the samples do not bundle: %v", p.Slug, err)
	}
	if diags := compiler.CheckApp(dir, compiler.CheckOptions{}); len(diags) > 0 {
		lines := make([]string, 0, len(diags))
		for _, d := range diags {
			lines = append(lines, strings.ReplaceAll(d.String(), dir+string(filepath.Separator), ""))
		}
		t.Fatalf("%s: gx check on the samples:\n%s", p.Slug, strings.Join(lines, "\n"))
	}
	build := exec.Command("go", "build", "./...")
	build.Dir = dir
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("%s: go build on the samples: %v\n%s", p.Slug, err, out)
	}
	if exportReport != "" {
		// The page shows the report of an export that must fail.
		_, err := exporter.Export(context.Background(), exporter.Options{
			Dir:   dir,
			Out:   filepath.Join(t.TempDir(), "dist"),
			Index: func(context.Context, string, string) error { return nil },
		})
		if err == nil || err.Error() != exportReport {
			t.Errorf("%s: the page shows this export report\n%s\nand gx export gives\n%v", p.Slug, exportReport, err)
		}
	}
	if len(gets) == 0 {
		return
	}
	bin := execname.Name(filepath.Join(t.TempDir(), "app"))
	build = exec.Command("go", "build", "-o", bin, "./cmd/app")
	build.Dir = dir
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("%s: build the app: %v\n%s", p.Slug, err, out)
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	app := exec.Command(bin)
	app.Dir = dir
	app.Env = append(os.Environ(), "GX_DEV_ADDR="+addr)
	if err := app.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = app.Process.Kill()
		_, _ = app.Process.Wait()
	}()
	for _, b := range gets {
		path := strings.TrimSpace(strings.TrimPrefix(b.Title, "GET "))
		body, status, err := fetch("http://" + addr + path)
		if err != nil {
			t.Fatalf("%s: GET %s: %v", p.Slug, path, err)
		}
		if status != http.StatusOK {
			t.Fatalf("%s: GET %s = %d\n%s", p.Slug, path, status, body)
		}
		for _, line := range strings.Split(b.Code, "\n") {
			if line = strings.TrimSpace(line); line != "" && !strings.Contains(body, line) {
				t.Errorf("%s: GET %s does not hold %q\n%s", p.Slug, path, line, body)
			}
		}
	}
}

// fetch reads one address of a sample app; the app can still be starting.
func fetch(url string) (string, int, error) {
	deadline := time.Now().Add(20 * time.Second)
	for {
		resp, err := http.Get(url)
		if err == nil {
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			return string(body), resp.StatusCode, err
		}
		if time.Now().After(deadline) {
			return "", 0, fmt.Errorf("the app did not start: %w", err)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
