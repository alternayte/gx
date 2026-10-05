package docscheck_test

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/analyze"
	"github.com/alternayte/gx/internal/compiler"
	"github.com/alternayte/gx/internal/docscheck"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

// sampleModule writes files into a module that replaces gx with this
// repository.
func sampleModule(t *testing.T, repo string, files map[string]string) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	files["go.mod"] = "module app\n\ngo 1.25.0\n\nrequire github.com/alternayte/gx v0.0.0\n\nreplace github.com/alternayte/gx => " + filepath.ToSlash(repo) + "\n"
	for rel, body := range files {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// TestREQ_DOC_03_DiagnosticPages covers the page of every diagnostic code:
// it has a cause, an example and a fix, and the example is true. The test
// builds the files of the example, runs the command the page names, and
// compares the diagnostics of that code with the output on the page
// (REQ-DOC-03).
func TestREQ_DOC_03_DiagnosticPages(t *testing.T) {
	repo := repoRoot(t)
	pages, err := docscheck.Pages(filepath.Join(docscheck.ContentDir(repo), "errors"))
	if err != nil {
		t.Fatal(err)
	}
	bySlug := map[string]docscheck.Page{}
	for _, p := range pages {
		bySlug[p.Slug] = p
	}
	if len(pages) != len(compiler.Catalog) {
		t.Errorf("docs/content/errors holds %d pages for %d codes", len(pages), len(compiler.Catalog))
	}
	index, err := os.ReadFile(filepath.Join(docscheck.ContentDir(repo), "reference", "diagnostics.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, info := range compiler.Catalog {
		info := info
		page, ok := bySlug[info.Code]
		if !ok {
			t.Errorf("no page for %s", info.Code)
			continue
		}
		if link := "(/errors/" + info.Code + "/)"; !strings.Contains(string(index), link) {
			t.Errorf("the diagnostics index does not link to %s", link)
		}
		t.Run(info.Code, func(t *testing.T) {
			t.Parallel()
			if want := info.Code + ": " + info.Title; page.Meta["title"] != want {
				t.Errorf("title = %q, want %q", page.Meta["title"], want)
			}
			if page.Meta["description"] == "" || page.Meta["code"] != info.Code {
				t.Errorf("frontmatter = %v", page.Meta)
			}
			for _, section := range []string{"Cause", "Example", "Fix"} {
				if len(page.Section(section)) < 20 {
					t.Errorf("section %q is missing or empty", section)
				}
			}

			files := map[string]string{}
			command, shown := "", ""
			for _, b := range page.Blocks() {
				switch {
				case b.Lang == "text" && strings.HasPrefix(b.Title, "gx "):
					command, shown = b.Title, b.Code
				case b.Title != "":
					files[b.Title] = b.Code
				}
			}
			if len(files) == 0 || shown == "" {
				t.Fatalf("the example has no file or no output block")
			}
			dir := sampleModule(t, repo, files)
			var got []string
			switch command {
			case "gx check":
				for _, d := range compiler.CheckApp(dir, compiler.CheckOptions{}) {
					if d.Code == info.Code {
						got = append(got, d.String())
					}
				}
			case "gx lint":
				findings, err := analyze.Lint(dir)
				if err != nil {
					t.Fatal(err)
				}
				for _, f := range findings {
					if f.Code == info.Code {
						got = append(got, fmt.Sprintf("%s:%d:%d: %s: %s", f.File, f.Line, f.Col, f.Code, f.Message))
					}
				}
			default:
				t.Fatalf("the output block names %q; a page uses gx check or gx lint", command)
			}
			for i := range got {
				got[i] = strings.ReplaceAll(filepath.ToSlash(got[i]), filepath.ToSlash(dir)+"/", "")
			}
			sort.Strings(got)
			if len(got) == 0 {
				t.Fatalf("the example does not give %s", info.Code)
			}
			if want := strings.TrimRight(shown, "\n"); strings.Join(got, "\n") != want {
				t.Errorf("the page shows\n%s\nand %s reports\n%s", want, command, strings.Join(got, "\n"))
			}
		})
	}
}
