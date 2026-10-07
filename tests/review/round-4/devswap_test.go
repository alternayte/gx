package round4_test

import (
	"bufio"
	"net/http"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"
)

// counterGx is a component with a signal and a bound input.
const counterGx = "package site\n\nsignals {\n  Qty int = 1\n}\n\n<div class=\"counter\">\n  <input type=\"number\" bind:value={$Qty} />\n</div>\n"

var instanceRe = regexp.MustCompile(`data-gx-instance="([^"]*)"`)

// instances returns the signal namespaces of the component instances of a
// page, in document order.
func instances(page string) []string {
	var out []string
	for _, m := range instanceRe.FindAllStringSubmatch(page, -1) {
		out = append(out, m[1])
	}
	return out
}

// REQ-DEV-02: a markup edit swaps into the running app, and the browser
// "keeps signals, input values, island state and scroll". Acceptance:
// "typed input value and signal value survive a markup edit".
//
// Defect: the signal namespace of a component with no key= holds the line
// and the column of its tag in the parent template
// (internal/compiler/codegen.go callKeyExpr: el.At.Line*1000 + el.At.Col).
// A markup edit that adds one line above the tag gives the instance a new
// namespace (site.Counter.4001 becomes site.Counter.5001). The browser
// holds the value of the signal under the old namespace, and the morphed
// input binds to the new one, so the signal and the typed input value go
// back to the first value. The browser test of the ledger
// (tests/e2e/dev.spec.ts) edits a page whose instances have key=, and its
// edit keeps every line, so it does not see this.
func TestREQ_DEV_02_SwapKeepsTheSignalsOfAComponentBelowTheEdit(t *testing.T) {
	d := startDevApp(t, map[string]string{
		"site/Home.gx":    "package site\n\n<h1>Home one</h1>\n<Counter />\n",
		"site/Counter.gx": counterGx,
	}, "Home one")
	before := instances(d.page())
	if len(before) != 1 {
		t.Fatalf("the page has %d component instances, want 1", len(before))
	}
	// The edit is markup only: one new paragraph above the counter.
	d.write("site/Home.gx", "package site\n\n<h1>Home two</h1>\n<p>A new line of text.</p>\n<Counter />\n")
	if name, _, data := d.next(60 * time.Second); name != "reload" {
		t.Fatalf("the edit gave the event %s %s, want reload", name, data)
	}
	after := instances(d.waitPage("Home two", 30*time.Second))
	if len(after) != 1 {
		t.Fatalf("the page has %d component instances after the edit, want 1", len(after))
	}
	if after[0] != before[0] {
		t.Fatalf("the markup edit moved the signals of the counter from the namespace %q to %q: the browser loses the signal value and the typed input value", before[0], after[0])
	}
}

// REQ-DEV-03: a change that cannot swap rebuilds, and the page then shows
// the app that a full build gives. REQ-ACT-06: two instances of a
// signal-bearing component keep separate signals.
//
// Defect: compiler.Session regenerates only the edited .gx file after a
// change that keeps the file signature (session.go incremental). The
// generated code of another file can depend on the body of the edited
// file: a component is "scoped" when its body renders a signal-bearing
// component, and a caller passes GxKey only to a scoped component. Mid
// gets a <Counter /> by a markup edit. The app refuses the swap (MidProps
// has a new field), gx dev rebuilds, and the build uses the stale
// Home_gx.go of the session, which passes no key to the two Mid tags. The
// two counters then share one signal namespace: a change of one changes
// the other. A build from a new session (gx generate) gives two
// namespaces.
func TestREQ_DEV_03_RebuildGeneratesTheCallersOfTheChangedComponent(t *testing.T) {
	d := startDevApp(t, map[string]string{
		"site/Home.gx":    "package site\n\n<h1>Home one</h1>\n<Mid key=\"a\" />\n<Mid key=\"b\" />\n",
		"site/Mid.gx":     "package site\n\n<section>\n  <p>mid one</p>\n</section>\n",
		"site/Counter.gx": counterGx,
	}, "Home one")
	d.write("site/Mid.gx", "package site\n\n<section>\n  <p>mid two</p>\n  <Counter />\n</section>\n")
	if name, _, data := d.next(120 * time.Second); name != "reload" {
		t.Fatalf("the edit gave the event %s %s, want reload", name, data)
	}
	got := instances(d.waitPage("mid two", 60*time.Second))
	if len(got) != 2 {
		t.Fatalf("the page has %d counters, want 2: %v", len(got), got)
	}
	if got[0] == got[1] {
		t.Fatalf("the two counters of <Mid key=\"a\" /> and <Mid key=\"b\" /> share the signal namespace %q after the rebuild of gx dev", got[0])
	}
}

// REQ-DEV-02 allows a swap only for a change that keeps the props, and
// REQ-AUT-18 requires that the committed generated code builds with go
// build alone.
//
// Defect: the same stale analysis, on the swap path. Mid loses its
// <Counter /> by a markup edit, so the generated MidProps loses the GxKey
// field. The file signature of Mid.gx is the same, so gx dev swaps, and it
// writes the new Mid_gx.go only. Home_gx.go on the disk still sets
// MidProps.GxKey: the tree that gx dev leaves does not compile.
func TestREQ_AUT_18_SwapLeavesGeneratedCodeThatBuilds(t *testing.T) {
	d := startDevApp(t, map[string]string{
		"site/Home.gx":    "package site\n\n<h1>Home one</h1>\n<Mid key=\"a\" />\n<Mid key=\"b\" />\n",
		"site/Mid.gx":     "package site\n\n<section>\n  <p>mid one</p>\n  <Counter />\n</section>\n",
		"site/Counter.gx": counterGx,
	}, "Home one")
	d.write("site/Mid.gx", "package site\n\n<section>\n  <p>mid two</p>\n</section>\n")
	if name, _, data := d.next(120 * time.Second); name != "reload" {
		t.Fatalf("the edit gave the event %s %s, want reload", name, data)
	}
	d.waitPage("mid two", 60*time.Second)
	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = d.dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("gx dev showed the edit, and the generated code that it left on the disk does not build: %v\n%s", err, out)
	}
}

// overlayOnConnect opens a new dev channel and reports whether the first
// message is an overlay: gx dev sends the current error to a browser that
// connects.
func overlayOnConnect(t *testing.T, addr string) (bool, string) {
	t.Helper()
	res, err := http.Get("http://" + addr + "/_gx/dev")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	got := make(chan string, 1)
	go func() {
		sc := bufio.NewScanner(res.Body)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		event := ""
		for sc.Scan() {
			line := sc.Text()
			if strings.HasPrefix(line, "event: ") {
				event = strings.TrimPrefix(line, "event: ")
			}
			if strings.HasPrefix(line, "data: ") && event == "overlay" {
				got <- strings.TrimPrefix(line, "data: ")
				return
			}
		}
	}()
	select {
	case data := <-got:
		return true, data
	case <-time.After(1500 * time.Millisecond):
		return false, ""
	}
}

// REQ-DEV-06: the overlay shows a compile error, and "the page recovers
// when the error is fixed". REQ-DEV-03: a Go change rebuilds.
//
// Defect: a Go file has a compile error, so the rebuild fails and the
// overlay shows it. The next markup edit of a .gx file swaps into the old
// app, and the swap clears the error of gx dev (devserver.go swap:
// s.setErr(nil), then a reload event). The Go error is still there. The
// overlay is gone, the browser shows the old app, and nothing tells the
// developer that the app on the screen is not the code on the disk.
func TestREQ_DEV_06_SwapKeepsTheOverlayOfABuildThatStillFails(t *testing.T) {
	d := startDevApp(t, map[string]string{
		"site/Home.gx": "package site\n\n<h1>Home one</h1>\n",
	}, "Home one")
	d.write("cmd/app/main.go", devMain+"\nvar broken = noSuchName\n")
	name, _, data := d.next(120 * time.Second)
	if name != "overlay" || !strings.Contains(data, "noSuchName") {
		t.Fatalf("the Go error gave the event %s %s, want an overlay with noSuchName", name, data)
	}
	if ok, _ := overlayOnConnect(t, d.addr); !ok {
		t.Fatal("a new browser gets no overlay for the build error")
	}
	// A markup edit. The Go error stays.
	d.write("site/Home.gx", "package site\n\n<h1>Home two</h1>\n")
	// gx dev handles the edit: a swap takes some milliseconds, and a
	// rebuild that fails takes less than the wait.
	d.settle(8 * time.Second)
	if ok, _ := overlayOnConnect(t, d.addr); !ok {
		t.Fatal("after a markup edit, gx dev shows no overlay, and cmd/app/main.go still has the compile error \"undefined: noSuchName\"")
	}
}

// REQ-DEV-04: the dev symbol table holds the package-level symbols.
// REQ-DEV-05, P7: interpreted and compiled code render the same bytes.
//
// Defect: the generated table writes an untyped float constant as the
// shortest decimal text of its float64 value (internal/compiler/symbols.go
// constantExpr: gx.DevFloat("0.3333333333333333") for 1.0 / 3). Go keeps
// an untyped constant exact, so Third * 3 is 1 in compiled code. The
// interpreter multiplies the rounded decimal and renders
// 0.9999999999999999. The value on the page changes after an edit of
// another line.
func TestREQ_DEV_05_UntypedFloatConstantOfTheSymbolTable(t *testing.T) {
	home := func(title string) string {
		return "package site\n\n<h1>" + title + "</h1>\n<p id=\"v\">{Third * 3}</p>\n"
	}
	d := startDevApp(t, map[string]string{
		"site/consts.go": "package site\n\n// Third is an untyped constant.\nconst Third = 1.0 / 3\n",
		"site/Home.gx":   home("Home one"),
	}, "Home one")
	const want = `<p id="v">1</p>`
	if page := d.page(); !strings.Contains(page, want) {
		t.Fatalf("the compiled page lacks %s:\n%s", want, page)
	}
	d.write("site/Home.gx", home("Home two"))
	if name, _, data := d.next(120 * time.Second); name != "reload" {
		t.Fatalf("the edit gave the event %s %s, want reload", name, data)
	}
	if page := d.waitPage("Home two", 60*time.Second); !strings.Contains(page, want) {
		t.Fatalf("after the edit of the heading the page lacks %s; the interpreted code renders another value:\n%s", want, page)
	}
}
