package islands_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
	"github.com/alternayte/gx/internal/islands"
)

func moduleWithGx(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repo := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	return "module app\n\ngo 1.25.0\n\nrequire github.com/alternayte/gx v0.0.0\n\nreplace github.com/alternayte/gx => " + filepath.ToSlash(repo) + "\n"
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

// The shared module stands for a chart library: two islands import it.
const sharedLib = `export const marker = "SHARED-LIBRARY-CODE";

export function draw(el: HTMLElement, label: string, values: number[]): void {
  el.textContent = marker + " " + label + " " + values.join(",");
}
`

func twoIslands(t *testing.T) string {
	return writeTree(t, map[string]string{
		"go.mod":          moduleWithGx(t),
		"lib/chartlib.ts": sharedLib,
		"dash/props.go":   "package dash\n\ntype RevenueChartProps struct {\n\tData []int `json:\"data\"`\n}\n\ntype UsersChartProps struct {\n\tData []int `json:\"data\"`\n}\n",
		"dash/RevenueChart.ts": `import { draw } from "../lib/chartlib";
import type { Props } from "./RevenueChart.props";

export default (el: HTMLElement, { data }: Props) => draw(el, "revenue-island", data);
`,
		"dash/UsersChart.ts": `import { draw } from "../lib/chartlib";
import type { Props } from "./UsersChart.props";

export default (el: HTMLElement, { data }: Props) => draw(el, "users-island", data);
`,
	})
}

var hashedName = regexp.MustCompile(`-[0-9A-Z]{8}\.js$`)

func TestREQ_ISL_03_SharedCodeIsOneChunk(t *testing.T) {
	dir := twoIslands(t)
	b, err := islands.Build(dir, islands.Options{Minify: true})
	if err != nil {
		t.Fatal(err)
	}
	revenue, users := b.Entries["app/dash/RevenueChart"], b.Entries["app/dash/UsersChart"]
	if revenue == "" || users == "" || len(b.Entries) != 2 {
		t.Fatalf("entries = %v, want the two islands", b.Entries)
	}
	if !strings.HasPrefix(revenue, "app/dash/RevenueChart-") {
		t.Fatalf("entry = %s", revenue)
	}
	var holders []string
	for name, body := range b.Files {
		if !hashedName.MatchString(name) {
			t.Errorf("file %s has no content hash in its name", name)
		}
		if strings.Contains(string(body), "SHARED-LIBRARY-CODE") {
			holders = append(holders, name)
		}
	}
	if len(holders) != 1 || !strings.HasPrefix(holders[0], "chunks/") {
		t.Fatalf("the shared code is in %v, want one file under chunks/", holders)
	}
	chunk := strings.TrimPrefix(holders[0], "chunks/")
	for id, entry := range map[string]string{"revenue-island": revenue, "users-island": users} {
		body := string(b.Files[entry])
		if !strings.Contains(body, id) {
			t.Errorf("%s lacks its own code %q:\n%s", entry, id, body)
		}
		if !strings.Contains(body, "import") || !strings.Contains(body, "chunks/"+chunk) {
			t.Errorf("%s does not import the shared chunk %s:\n%s", entry, chunk, body)
		}
		if !strings.Contains(body, "export{") && !strings.Contains(body, "export {") {
			t.Errorf("%s is not an ES module:\n%s", entry, body)
		}
	}
	if len(b.Files) != 3 {
		t.Fatalf("files = %d, want two entries and one chunk", len(b.Files))
	}
}

// The name of a file follows its content: an edit of one island renames
// that island only, so a browser keeps the other files.
func TestREQ_ISL_03_ContentHashedNames(t *testing.T) {
	dir := twoIslands(t)
	before, err := islands.Build(dir, islands.Options{Minify: true})
	if err != nil {
		t.Fatal(err)
	}
	again, err := islands.Build(dir, islands.Options{Minify: true})
	if err != nil {
		t.Fatal(err)
	}
	if string(islands.Generate(before)) != string(islands.Generate(again)) {
		t.Fatal("two builds of the same source differ")
	}
	path := filepath.Join(dir, "dash", "UsersChart.ts")
	src, _ := os.ReadFile(path)
	if err := os.WriteFile(path, []byte(strings.Replace(string(src), "users-island", "users-island-v2", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	after, err := islands.Build(dir, islands.Options{Minify: true})
	if err != nil {
		t.Fatal(err)
	}
	if before.Entries["app/dash/UsersChart"] == after.Entries["app/dash/UsersChart"] {
		t.Fatal("the edited island kept its file name")
	}
	if before.Entries["app/dash/RevenueChart"] != after.Entries["app/dash/RevenueChart"] {
		t.Fatal("the other island changed its file name")
	}
}

func TestREQ_ISL_03_DevBuildHasSourceMaps(t *testing.T) {
	b, err := islands.Build(twoIslands(t), islands.Options{})
	if err != nil {
		t.Fatal(err)
	}
	entry := b.Entries["app/dash/RevenueChart"]
	if _, ok := b.Files[entry+".map"]; !ok {
		t.Fatalf("no source map for %s", entry)
	}
	if !strings.Contains(string(b.Files[entry]), "//# sourceMappingURL=") {
		t.Fatalf("the entry does not name its source map")
	}
}

func TestREQ_ISL_03_SyntaxErrorHasAPosition(t *testing.T) {
	dir := twoIslands(t)
	path := filepath.Join(dir, "dash", "UsersChart.ts")
	if err := os.WriteFile(path, []byte("export default (el: HTMLElement) => {\n  const = 1;\n};\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := islands.Build(dir, islands.Options{})
	berr, ok := err.(*islands.Error)
	if !ok || len(berr.Messages) == 0 {
		t.Fatalf("err = %v, want an *islands.Error", err)
	}
	m := berr.Messages[0]
	if filepath.Base(m.File) != "UsersChart.ts" || m.Line != 2 || m.Col != 9 {
		t.Fatalf("message = %+v, want UsersChart.ts:2:9", m)
	}
	if !strings.Contains(err.Error(), "UsersChart.ts:2:9: ") {
		t.Fatalf("error text = %q", err.Error())
	}
}

func TestREQ_ISL_03_NoIslandWritesNoPackage(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":       moduleWithGx(t),
		"home/Home.gx": "package home\n\n<main>Home</main>\n",
	})
	wrote, err := islands.Write(dir, islands.Options{})
	if err != nil || wrote {
		t.Fatalf("Write = %v, %v", wrote, err)
	}
	if _, err := os.Stat(islands.Path(dir)); err == nil {
		t.Fatal("an app with no island got a gxislands package")
	}
}

// The production binary holds the bundle: the app serves each file from
// memory, with the directory of the source gone.
func TestREQ_ISL_03_BundleIsEmbeddedInTheBinary(t *testing.T) {
	dir := twoIslands(t)
	extra := map[string]string{
		"dash/Page.gx": "package dash\n\n<section><RevenueChart data={[]int{1, 2}} /><UsersChart data={[]int{3}} /></section>\n",
		"main.go": `package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"

	"app/dash"
	"app/gxislands"
	gx "github.com/alternayte/gx"
)

func main() {
	gx.SetIslands(gxislands.Bundle())
	app := gx.New(gx.Config{})
	page := gx.String(dash.Page(dash.PageProps{}))
	fmt.Println(page)
	srv := httptest.NewServer(app)
	defer srv.Close()
	for _, m := range regexp.MustCompile(` + "`src=\"([^\"]+)\"`" + `).FindAllStringSubmatch(page, -1) {
		res, err := http.Get(srv.URL + m[1])
		if err != nil {
			panic(err)
		}
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		fmt.Println(res.StatusCode, res.Header.Get("Content-Type"), res.Header.Get("Cache-Control"), len(body) > 0)
		chunk := regexp.MustCompile(` + "`chunks/[A-Za-z0-9_-]+\\.js`" + `).FindString(string(body))
		res, err = http.Get(srv.URL + "/_gx/islands/" + chunk)
		if err != nil {
			panic(err)
		}
		body, _ = io.ReadAll(res.Body)
		res.Body.Close()
		fmt.Println(res.StatusCode, regexp.MustCompile("SHARED-LIBRARY-CODE").Match(body))
	}
	res, _ := http.Get(srv.URL + "/_gx/islands/none.js")
	fmt.Println(res.StatusCode)
}
`,
	}
	for rel, content := range extra {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
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
	if wrote, err := islands.Write(dir, islands.Options{Minify: true}); err != nil || !wrote {
		t.Fatalf("Write = %v, %v", wrote, err)
	}
	if wrote, err := islands.Write(dir, islands.Options{Minify: true}); err != nil || wrote {
		t.Fatalf("second Write = %v, %v, want no new file", wrote, err)
	}
	if out, err := exec.Command("gofmt", "-l", islands.Path(dir)).CombinedOutput(); err != nil || len(out) != 0 {
		t.Fatalf("gofmt -l: %v %s", err, out)
	}
	bin := filepath.Join(t.TempDir(), "app")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = dir
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	if err := os.RemoveAll(filepath.Join(dir, "dash")); err != nil {
		t.Fatal(err)
	}
	run := exec.Command(bin)
	run.Dir = t.TempDir()
	out, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("run: %v\n%s", err, out)
	}
	var lines []string
	for _, line := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) != 6 {
		t.Fatalf("output:\n%s", out)
	}
	if !regexp.MustCompile(`<gx-island name="app/dash/RevenueChart" props="[^"]+" src="/_gx/islands/app/dash/RevenueChart-[0-9A-Z]{8}\.js"><div data-gx-island-root data-ignore-morph></div></gx-island><gx-island name="app/dash/UsersChart" props="[^"]+" src="/_gx/islands/app/dash/UsersChart-[0-9A-Z]{8}\.js">`).MatchString(lines[0]) {
		t.Fatalf("page = %s", lines[0])
	}
	want := "200 text/javascript; charset=utf-8 public, max-age=31536000, immutable true"
	for _, i := range []int{1, 3} {
		if lines[i] != want {
			t.Fatalf("line %d = %q, want %q", i, lines[i], want)
		}
		if lines[i+1] != "200 true" {
			t.Fatalf("line %d = %q, want the shared chunk", i+1, lines[i+1])
		}
	}
	if lines[5] != "404" {
		t.Fatalf("unknown file = %q, want 404", lines[5])
	}
}
