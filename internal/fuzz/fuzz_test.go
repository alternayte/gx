package fuzz_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alternayte/gx/internal/apprun"
	"github.com/alternayte/gx/internal/fuzz"
	"github.com/alternayte/gx/internal/scaffold"
)

func repoRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

var shared struct {
	once sync.Once
	err  error
	dir  string
}

const crashGx = `package crash

props {
  // Name is the name of the person. The component shows its first letter.
  Name string
}

<span class="rounded-full border border-border px-2">{p.Name[:1]}</span>
`

const crashFixtures = `package crash

import "github.com/alternayte/gx"

// CrashFixtures are the examples of Crash in the dev gallery.
var CrashFixtures = gx.Fixtures[CrashProps]{
	"Default": {Name: "Ada"},
}
`

const rowGx = `package row

props {
  // Title is the heading of the row.
  Title string
  // Count is the number of items.
  Count int
  // Open shows the content.
  Open bool = false
  // Tags are the labels of the row.
  Tags []string = nil
  // Children is the content.
  Children gx.Node
}

<section>
  <h2>{p.Title}</h2>
  <p>{p.Count}</p>
  <ul>
    for _, tag := range p.Tags {
      <li>{tag}</li>
    }
  </ul>
  if p.Open {
    <div>{p.Children}</div>
  }
</section>
`

const badGx = `package bad

props {
  // Attrs are the attributes of the image.
  Attrs gx.Attrs = nil
}

<div><img src="data:image/gif;base64,R0lGODlhAQABAAAAACw=" width="20" height="20" {...p.Attrs} /></div>
`

const cellsGx = `package cells

props {
  // Children is the cells of the row.
  Children gx.Node
}

<table><tbody><tr>{p.Children}</tr></tbody></table>
`

const needGx = `package need

import "fmt"

props {
  // View is the value that the component shows.
  View fmt.Stringer
}

<p>{p.View.String()}</p>
`

func write(dir, rel, content string) error {
	return os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), []byte(content), 0o644)
}

// setup makes one app with five components: Crash panics on an empty
// string, Row has props of several kinds, Bad has an image with no text
// alternative, Cells has a slot that takes table cells only, and Need has
// a required prop of an interface type.
func setup() error {
	parent, err := os.MkdirTemp("", "gx-fuzz-test-")
	if err != nil {
		return err
	}
	dir := filepath.Join(parent, "acme")
	shared.dir = dir
	if _, err := scaffold.Init(scaffold.Options{
		Dir: dir, Module: "example.com/acme", Adapter: "datastar", Version: "0.1.0", Replace: repoRoot(),
	}); err != nil {
		return err
	}
	for _, name := range []string{"ui/crash/Crash", "ui/row/Row", "ui/bad/Bad", "ui/cells/Cells", "ui/need/Need"} {
		if _, err := scaffold.New(dir, "component", name); err != nil {
			return err
		}
	}
	for rel, content := range map[string]string{
		"ui/crash/Crash.gx":          crashGx,
		"ui/crash/Crash.fixtures.go": crashFixtures,
		"ui/row/Row.gx":              rowGx,
		"ui/bad/Bad.gx":              badGx,
		"ui/cells/Cells.gx":          cellsGx,
		"ui/need/Need.gx":            needGx,
	} {
		if err := write(dir, rel, content); err != nil {
			return err
		}
	}
	for _, rel := range []string{"ui/row/Row.fixtures.go", "ui/bad/Bad.fixtures.go", "ui/need/Need.fixtures.go"} {
		if err := os.Remove(filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
			return err
		}
	}
	_, err = scaffold.Generate(dir)
	return err
}

func TestMain(m *testing.M) {
	// The app of the tests resolves the gx module through a replace.
	os.Setenv("GOFLAGS", "-mod=mod")
	code := m.Run()
	if shared.dir != "" {
		_ = os.RemoveAll(filepath.Dir(shared.dir))
	}
	os.Exit(code)
}

func app(t *testing.T) string {
	t.Helper()
	shared.once.Do(func() { shared.err = setup() })
	if shared.err != nil {
		t.Fatalf("setup: %v", shared.err)
	}
	return shared.dir
}

func run(t *testing.T, opt fuzz.Options) *fuzz.Result {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	opt.Dir = app(t)
	res, err := fuzz.Run(ctx, opt)
	if err != nil {
		t.Fatalf("gx fuzz %s: %v", opt.Component, err)
	}
	return res
}

// TestREQ_AI_11_FuzzFindsThePanic covers the acceptance of REQ-AI-11: the
// run finds the component that panics on an empty string, and the fixture
// that it prints gives the panic again.
func TestREQ_AI_11_FuzzFindsThePanic(t *testing.T) {
	dir := app(t)
	res := run(t, fuzz.Options{Component: "crash.Crash", Seed: 1, Sets: 3})
	if res.Components != 1 || res.Sets != 3 {
		t.Fatalf("the run rendered %d sets of %d components, want 3 of 1", res.Sets, res.Components)
	}
	var found *fuzz.Failure
	for i, f := range res.Failures {
		if f.Kind == "panic" && f.Index == 0 {
			found = &res.Failures[i]
		} else if f.Kind == "panic" {
			t.Errorf("set %d panics: %s", f.Index, f.Message)
		}
	}
	if found == nil {
		t.Fatalf("the run did not find the panic on the empty string: %+v", res.Failures)
	}
	if !strings.Contains(found.Message, "slice bounds out of range") {
		t.Errorf("the message does not name the panic: %s", found.Message)
	}
	var out bytes.Buffer
	res.Print(&out)
	entry := `"` + found.Name + `": ` + found.Fixture + `,`
	if !strings.Contains(out.String(), entry) || !strings.Contains(out.String(), "seed 1") {
		t.Fatalf("the output has no fixture entry %s and seed:\n%s", entry, out.String())
	}

	// The printed entry goes into the fixtures file. The gallery then
	// renders it, and the render panics again.
	fixtures := strings.Replace(crashFixtures, "\t\"Default\"", "\t"+entry+"\n\t\"Default\"", 1)
	if err := write(dir, "ui/crash/Crash.fixtures.go", fixtures); err != nil {
		t.Fatal(err)
	}
	defer write(dir, "ui/crash/Crash.fixtures.go", crashFixtures)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	running, err := apprun.Start(ctx, dir, "")
	if err != nil {
		t.Fatalf("the app with the printed fixture does not build: %v", err)
	}
	defer running.Stop()
	status := func(name string) int {
		resp, err := http.Get(running.Base + "/_gx/gallery/fixture?component=Crash&name=" + name)
		if err != nil {
			// The server ends the connection of a handler that panics.
			return 0
		}
		defer resp.Body.Close()
		return resp.StatusCode
	}
	if got := status("Default"); got != http.StatusOK {
		t.Fatalf("the fixture Default answers %d", got)
	}
	if got := status(found.Name); got == http.StatusOK || got == http.StatusNotFound {
		t.Fatalf("the printed fixture answers %d, want the panic", got)
	}
}

// TestREQ_AI_11_SeedRepeats covers the seed: two runs with one seed give
// the same prop sets, and the first three sets hold the empty value, a
// long string and markup characters in each string.
func TestREQ_AI_11_SeedRepeats(t *testing.T) {
	sets := func(seed uint64) string {
		res := run(t, fuzz.Options{Component: "Row", Seed: seed, Sets: 8})
		for _, f := range res.Failures {
			if f.Kind != "a11y" {
				t.Errorf("Row, set %d: %s", f.Index, f.Message)
			}
		}
		data, err := json.Marshal(res.Report)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	first, second, other := sets(42), sets(42), sets(43)
	if first != second {
		t.Fatalf("two runs with one seed differ:\n%s\n%s", first, second)
	}
	if first == other {
		t.Fatal("a different seed gives the same sets")
	}
	var report struct {
		Targets []struct {
			Cases []struct {
				Fixture, HTML string
			}
		}
	}
	if err := json.Unmarshal([]byte(first), &report); err != nil || len(report.Targets) != 1 || len(report.Targets[0].Cases) != 8 {
		t.Fatalf("the report has not 8 sets of one component: %v\n%s", err, first)
	}
	cases := report.Targets[0].Cases
	if cases[0].Fixture != "{}" {
		t.Errorf("set 0 is not the empty value: %s", cases[0].Fixture)
	}
	if !strings.Contains(cases[1].Fixture, `Title: "Lorem ipsum`) || len(cases[1].Fixture) < 2000 {
		t.Errorf("set 1 has no long string: %.200s", cases[1].Fixture)
	}
	if !strings.Contains(cases[2].Fixture, `Title: "<b>\"x\" & 'y'</b><script>`) {
		t.Errorf("set 2 has no markup characters: %s", cases[2].Fixture)
	}
	if strings.Contains(cases[2].HTML, "<script>") || !strings.Contains(cases[2].HTML, "&lt;script&gt;") {
		t.Errorf("set 2 renders the markup characters as markup:\n%s", cases[2].HTML)
	}
}

// TestREQ_AI_11_AxeRuns covers the audit: a component with an image that
// has no text alternative fails with the axe rule.
func TestREQ_AI_11_AxeRuns(t *testing.T) {
	res := run(t, fuzz.Options{Component: "Bad", Seed: 1, Sets: 1})
	if len(res.Failures) != 1 || res.Failures[0].Kind != "a11y" || !strings.Contains(res.Failures[0].Message, "image-alt") {
		t.Fatalf("the run did not report image-alt: %+v", res.Failures)
	}
}

// TestREQ_AI_11_InputsOfACaller covers the two inputs that no caller
// gives. Text in a slot for table cells is not a defect of the component.
// A component with a required prop that the run cannot make is skipped
// with a message, and is not rendered with nil.
func TestREQ_AI_11_InputsOfACaller(t *testing.T) {
	res := run(t, fuzz.Options{Component: "Cells", Seed: 5, Sets: 12})
	slots := false
	for _, target := range res.Report.Targets {
		for _, c := range target.Cases {
			slots = slots || (c.Slots && fuzz.TreeDefect(c.HTML) != "")
		}
	}
	if !slots {
		t.Fatal("no set has text in the slot of the row; the test needs one")
	}
	for _, f := range res.Failures {
		if f.Kind != "a11y" {
			t.Errorf("Cells, set %d: %s", f.Index, f.Message)
		}
	}

	res = run(t, fuzz.Options{Component: "Need", Seed: 5, Sets: 3})
	if res.Components != 0 || len(res.Failures) != 0 || len(res.Skipped) != 1 ||
		!strings.Contains(res.Skipped[0], "example.com/acme/ui/need.Need: gx fuzz cannot make a value for the required prop View") {
		t.Fatalf("Need: %d components, failures %+v, skipped %v", res.Components, res.Failures, res.Skipped)
	}
}
