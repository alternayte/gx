package docscheck_test

import (
	"bytes"
	"go/ast"
	"go/doc"
	"go/parser"
	"go/token"
	"image/gif"
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
		"guides/widgets":              "Guides", // widgets for a host page
		"guides/chart-js":             "Guides", // an npm chart library in an island
		"guides/app-tools":            "Guides", // actions as tools for agents
		"guides/security":             "Guides", // the security rules
		"guides/islands":              "Guides", // islands and web components
		"guides/adapters":             "Guides", // Datastar and htmx

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

// TestREQ_DOC_05_Readme covers the README: the quick start in three
// commands, the GIF of the dev loop and the Cart example. The sample check
// compiles the Cart example (REQ-DOC-05). A person looks at the page at
// gate G4.
func TestREQ_DOC_05_Readme(t *testing.T) {
	repo := repoRoot(t)
	readme, err := docscheck.ReadPage(filepath.Join(repo, "README.md"), "README")
	if err != nil {
		t.Fatal(err)
	}
	var quick *docscheck.Block
	files := map[string]string{}
	blocks := readme.Blocks()
	for i := range blocks {
		b := &blocks[i]
		if b.Lang == "sh" && quick == nil {
			quick = b
		}
		if b.Title != "" {
			files[b.Title] = b.Code
		}
	}
	if quick == nil {
		t.Fatal("the README has no shell block")
	}
	want := []string{
		"go install github.com/alternayte/gx/cmd/gx@latest",
		"gx init acme",
		"cd acme && go run ./cmd/gx dev",
	}
	if got := strings.Split(strings.TrimSpace(quick.Code), "\n"); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("the quick start is %q, want the three commands %q", got, want)
	}
	if at := strings.Index(readme.Body, "## Quick start"); at < 0 || at > 1200 {
		t.Errorf("the quick start is not near the top of the README (offset %d)", at)
	}

	// The Cart example: a signal, an action and a fragment, in three files.
	for file, parts := range map[string][]string{
		"cart/Cart.gx":        {"signals {", "bind:value={$Qty}", "on:click={route.Add{}}", "#total(total int)"},
		"cart/route/route.go": {"gx.Route `POST /cart/add`", "`signal:\"qty\"`", "Rules() gx.Rules"},
		"cart/cart.go":        {"gx.Action(", "c.Patch(CartTotal("},
	} {
		for _, part := range parts {
			if !strings.Contains(files[file], part) {
				t.Errorf("the Cart example file %s lacks %q", file, part)
			}
		}
	}

	// The GIF of the dev loop is in the repository and has its frames.
	const gifPath = "docs/assets/dev-loop.gif"
	if !strings.Contains(readme.Body, "]("+gifPath+")") {
		t.Fatalf("the README does not show %s", gifPath)
	}
	raw, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(gifPath)))
	if err != nil {
		t.Fatal(err)
	}
	anim, err := gif.DecodeAll(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("%s is not a GIF: %v", gifPath, err)
	}
	if len(anim.Image) < 4 || len(raw) > 600<<10 {
		t.Errorf("%s has %d frames and %d bytes; want at least 4 frames and at most 600 KB", gifPath, len(anim.Image), len(raw))
	}
	if b := anim.Image[0].Bounds(); b.Dx() < 600 || b.Dy() < 300 {
		t.Errorf("%s is %dx%d; a reader cannot read it", gifPath, b.Dx(), b.Dy())
	}
	if !strings.Contains(readme.Body, "https://gx-docs.pages.dev") {
		t.Error("the README does not link to the docs site")
	}
}

// TestREQ_DOC_01_DeployJob covers the deploy of the docs site: a workflow
// exports the docs app with gx export and deploys the output to the
// Cloudflare Pages project, the site URL of the app is the address of that
// project, and the docs app has no action and no form, so the export can
// hold all of it (REQ-DOC-01). The browser suite exports the site and reads
// the result.
func TestREQ_DOC_01_DeployJob(t *testing.T) {
	repo := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(repo, ".github", "workflows", "docs.yml"))
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(raw)
	for _, want := range []string{
		"branches: [main]",
		"go run ./cmd/gx export -main . --out docs/dist docs",
		"uses: cloudflare/wrangler-action@v3",
		"pages deploy docs/dist --project-name=gx-docs",
		"${{ secrets.CLOUDFLARE_API_TOKEN }}",
		"${{ secrets.CLOUDFLARE_ACCOUNT_ID }}",
	} {
		if !strings.Contains(workflow, want) {
			t.Errorf("the deploy workflow lacks %q", want)
		}
	}
	// The export comes before the deploy.
	if strings.Index(workflow, "gx export") > strings.Index(workflow, "pages deploy") {
		t.Error("the workflow deploys before it exports")
	}
	// No secret value is in the file.
	for _, line := range strings.Split(workflow, "\n") {
		if strings.Contains(line, "apiToken:") && !strings.Contains(line, "secrets.") {
			t.Errorf("the workflow holds a token: %s", line)
		}
	}

	config, err := os.ReadFile(filepath.Join(repo, "docs", "gx.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(config), `url = "https://gx-docs.pages.dev"`) {
		t.Error("docs/gx.toml does not name the address of the Pages project")
	}

	// The docs site uses the content features only: it has no action and
	// no form.
	files, err := filepath.Glob(filepath.Join(repo, "docs", "site", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		// A generated file holds the text of the docs pages, which name
		// these calls.
		if strings.HasSuffix(file, "_test.go") || strings.HasSuffix(file, "_gx.go") {
			continue
		}
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, call := range []string{"gx.Action(", "gx.Form("} {
			if strings.Contains(string(src), call) {
				t.Errorf("%s calls %s; the docs site must export to static files", filepath.Base(file), call)
			}
		}
	}
}
