package docscheck_test

import (
	"go/ast"
	"go/doc"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/docscheck"
)

// TestREQ_DOC_02_PageInventory covers the sections of the docs site: the
// quick start, the tutorial, one guide for each shipped feature, the
// reference (API, command, diagnostics, grammar), the comparisons and the
// move from templ. Each page has a title, a description, a section and
// real content (REQ-DOC-02).
func TestREQ_DOC_02_PageInventory(t *testing.T) {
	repo := repoRoot(t)
	pages, err := docscheck.Pages(docscheck.ContentDir(repo))
	if err != nil {
		t.Fatal(err)
	}
	bySlug := map[string]docscheck.Page{}
	for _, p := range pages {
		bySlug[p.Slug] = p
	}
	want := map[string]string{
		"start/quick-start": "Start",

		"tutorial/a-page":              "Tutorial",
		"tutorial/a-parameter":         "Tutorial",
		"tutorial/an-action":           "Tutorial",
		"tutorial/a-form":              "Tutorial",
		"tutorial/registry-components": "Tutorial",
		"tutorial/ship":                "Tutorial",

		// One guide for each feature area of the specification that
		// release 0.1.0 ships.
		"guides/components":           "Guides", // authoring
		"guides/routing":              "Guides", // routes, pages, layouts, navigation
		"guides/actions-and-signals":  "Guides", // actions, fragments, signals
		"guides/forms":                "Guides", // forms and validation
		"guides/styling":              "Guides", // styling, icons, transitions
		"guides/registry":             "Guides", // the component registry
		"guides/agent-tools":          "Guides", // describe, MCP, AGENTS.md
		"guides/dev-loop-and-editors": "Guides", // dev loop, editors, Go tools
		"guides/content-sites":        "Guides", // content collections
		"guides/static-export":        "Guides", // static export
		"guides/security":             "Guides", // the security rules

		"reference/api":         "Reference",
		"reference/cli":         "Reference",
		"reference/grammar":     "Reference",
		"reference/diagnostics": "Reference",

		"compare/comparisons":        "Compare",
		"compare/migrate-from-templ": "Compare",
	}
	for slug, section := range want {
		p, ok := bySlug[slug]
		if !ok {
			t.Errorf("the docs have no page %s", slug)
			continue
		}
		if p.Meta["section"] != section {
			t.Errorf("%s is in section %q, want %q", slug, p.Meta["section"], section)
		}
		if p.Meta["title"] == "" || len(p.Meta["description"]) < 20 {
			t.Errorf("%s needs a title and a description: %v", slug, p.Meta)
		}
		if len(p.Body) < 800 {
			t.Errorf("%s holds %d bytes of content", slug, len(p.Body))
		}
	}
	// A page of a section that this list does not know is a page nobody
	// decided on.
	for _, p := range pages {
		if strings.HasPrefix(p.Slug, "components/") || strings.HasPrefix(p.Slug, "errors/") || p.Slug == "components" || p.Slug == "index" {
			continue
		}
		if _, ok := want[p.Slug]; !ok {
			t.Errorf("%s is not in the page inventory of this test", p.Slug)
		}
	}
	if _, ok := bySlug["components"]; !ok {
		t.Error("the registry gallery has no index page")
	}

	// The tutorial builds one app, step by step.
	for slug, section := range want {
		if section == "Tutorial" && bySlug[slug].Meta["sample"] != "tutorial" {
			t.Errorf("%s is not a step of the tutorial app", slug)
		}
	}

	// The comparisons name each alternative, and for each one what it
	// does better.
	compare := bySlug["compare/comparisons"].Body
	for _, name := range []string{"templ", "gomponents", "Next.js", "Rails with Hotwire"} {
		if !strings.Contains(compare, "\n## "+name+"\n") {
			t.Errorf("the comparisons page has no section %q", name)
		}
	}
}

// TestREQ_DOC_02_CommandReference covers the command reference: each
// command that `gx help` lists has a section with its usage (REQ-DOC-02).
func TestREQ_DOC_02_CommandReference(t *testing.T) {
	repo := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(docscheck.ContentDir(repo), "reference", "cli.md"))
	if err != nil {
		t.Fatal(err)
	}
	page := string(raw)
	for command := range gxCommands(t, repo) {
		section := "\n## gx " + command + "\n"
		at := strings.Index(page, section)
		if at < 0 {
			t.Errorf("the command reference has no section for gx %s", command)
			continue
		}
		rest := page[at+len(section):]
		if next := strings.Index(rest, "\n## "); next >= 0 {
			rest = rest[:next]
		}
		if !strings.Contains(rest, "```sh\ngx "+command) {
			t.Errorf("the section of gx %s shows no usage line", command)
		}
	}
}

// TestREQ_DOC_02_APIReference covers the reference of package gx: each
// exported function, type, constant and variable of the production API has
// an entry (REQ-DOC-02).
func TestREQ_DOC_02_APIReference(t *testing.T) {
	repo := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(docscheck.ContentDir(repo), "reference", "api.md"))
	if err != nil {
		t.Fatal(err)
	}
	page := string(raw)
	fset := token.NewFileSet()
	entries, err := os.ReadDir(repo)
	if err != nil {
		t.Fatal(err)
	}
	var files []*ast.File
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || strings.HasSuffix(name, "_gxdev.go") || name == "cxtable.go" {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(repo, name), nil, parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, file)
	}
	pkg, err := doc.NewFromFiles(fset, files, "github.com/alternayte/gx")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	need := func(heading string) {
		count++
		if !strings.Contains(page, "\n"+heading+"\n") {
			t.Errorf("the API reference has no entry %q; run just docs-gen", heading)
		}
	}
	for _, f := range pkg.Funcs {
		need("### func " + f.Name)
	}
	for _, typ := range pkg.Types {
		need("### type " + typ.Name)
		for _, f := range typ.Funcs {
			need("#### func " + f.Name)
		}
		for _, m := range typ.Methods {
			need("#### func (" + strings.TrimPrefix(m.Recv, "*") + ") " + m.Name)
		}
	}
	for _, v := range append(pkg.Consts, pkg.Vars...) {
		need("### " + strings.Join(v.Names, ", "))
	}
	if count < 150 {
		t.Fatalf("package gx gave %d symbols; the scan is wrong", count)
	}
	// go/doc lists a function under the type that it returns, so a
	// function has a heading of level three or four.
	for _, core := range []string{"func Page", "func Action", "func Form", "func Collect", "func Cx", "func CSP", "type Enum", "type Ctx"} {
		if !strings.Contains(page, "### "+core+"\n") {
			t.Errorf("the API reference lacks %q", core)
		}
	}
}

// TestREQ_DOC_06_HonestComparisons covers the shape of the comparisons
// page: it states what Gx does not have, and for each alternative it names
// at least three things that the alternative does better, what Gx does
// better, and when to choose the alternative (REQ-DOC-06). A person judges
// the content at gate G4.
func TestREQ_DOC_06_HonestComparisons(t *testing.T) {
	repo := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(docscheck.ContentDir(repo), "compare", "comparisons.md"))
	if err != nil {
		t.Fatal(err)
	}
	page := string(raw)
	section := func(heading string) string {
		at := strings.Index(page, "\n## "+heading+"\n")
		if at < 0 {
			t.Fatalf("the page has no section %q", heading)
		}
		rest := page[at+len(heading)+5:]
		if next := strings.Index(rest, "\n## "); next >= 0 {
			rest = rest[:next]
		}
		return rest
	}
	bullets := func(text, after string) int {
		at := strings.Index(text, after)
		if at < 0 {
			return 0
		}
		count := 0
		for _, line := range strings.Split(strings.TrimLeft(text[at+len(after):], "\n"), "\n") {
			if !strings.HasPrefix(line, "- ") {
				break
			}
			count++
		}
		return count
	}
	if n := strings.Count(section("What Gx does not have"), "\n- "); n < 4 {
		t.Errorf("the page names %d limits of Gx; a reader needs them before the comparisons", n)
	}
	for _, name := range []string{"templ", "gomponents", "Next.js", "Rails with Hotwire"} {
		text := section(name)
		short := strings.Fields(name)[0]
		if n := bullets(text, "**What "+short+" does better**"); n < 3 {
			t.Errorf("%s: %d points that it does better; want at least 3", name, n)
		}
		if n := bullets(text, "**What Gx does better**"); n < 2 {
			t.Errorf("%s: %d points that Gx does better; want at least 2", name, n)
		}
		if !strings.Contains(text, "**Choose "+short+" when**") {
			t.Errorf("%s: no advice on when to choose it", name)
		}
		// The section does not sell: no word of praise for Gx.
		for _, word := range []string{"best", "superior", "blazing", "revolution", "effortless"} {
			if strings.Contains(strings.ToLower(text), word) {
				t.Errorf("%s: the word %q is praise, not a fact", name, word)
			}
		}
	}
	if !strings.Contains(page, "\n## Which one\n") {
		t.Error("the page has no summary table")
	}
}
