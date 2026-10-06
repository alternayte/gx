package devserver_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/alternayte/gx/internal/devserver"
	"github.com/alternayte/gx/internal/testbudget"
)

// copyShop copies the example shop into a temp directory, so a test can
// edit its templates.
func copyShop(t *testing.T) string {
	t.Helper()
	repo := repoRoot(t)
	src := filepath.Join(repo, "examples", "shop")
	dst := t.TempDir()
	err := filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		if d.IsDir() {
			if d.Name() == ".gx" || d.Name() == "bin" {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if rel == "go.mod" {
			// The copy needs an absolute replace to the repository.
			data = []byte(strings.Replace(string(data), "replace github.com/alternayte/gx => ../..", "replace github.com/alternayte/gx => "+filepath.ToSlash(repo), 1))
		}
		return os.WriteFile(filepath.Join(dst, rel), data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dst
}

// devInfo is the answer of the dev-only info route of the app.
type devInfo struct {
	PID     int `json:"pid"`
	Swapped int `json:"swapped"`
}

func info(t *testing.T, addr string) devInfo {
	t.Helper()
	res, err := http.Get("http://" + addr + "/_gx/dev/info")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out devInfo
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out
}

func page(t *testing.T, url string) string {
	t.Helper()
	res, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	return string(body)
}

// reloadEvent is the data of a reload event of the dev channel.
type reloadEvent struct {
	Swap bool `json:"swap"`
}

func waitReload(t *testing.T, ch chan event, timeout time.Duration) reloadEvent {
	t.Helper()
	e := waitEvent(t, ch, "reload", timeout)
	var out reloadEvent
	_ = json.Unmarshal([]byte(e.data), &out)
	return out
}

// shopDev is a dev server on a copy of the example shop, with its about
// page as the template that a test edits.
type shopDev struct {
	t        *testing.T
	dir      string
	addr     string
	url      string
	ch       chan event
	original string
}

func startShopDev(t *testing.T) *shopDev {
	t.Helper()
	d := &shopDev{t: t, dir: copyShop(t), addr: freeAddr(t)}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() {
		if err := devserver.Run(ctx, devserver.Options{Dir: d.dir, Main: "./cmd/shop", Addr: d.addr, Log: os.Stderr}); err != nil {
			t.Logf("devserver: %v", err)
		}
	}()
	src, err := os.ReadFile(filepath.Join(d.dir, "About.gx"))
	if err != nil {
		t.Fatal(err)
	}
	d.original = string(src)
	d.url = "http://" + d.addr + "/about"
	getText(t, d.url, "A shop built with Gx.", 120*time.Second)
	d.ch = events(t, d.addr)
	return d
}

// write replaces the about template.
func (d *shopDev) write(text string) {
	d.t.Helper()
	if err := os.WriteFile(filepath.Join(d.dir, "About.gx"), []byte(text), 0o644); err != nil {
		d.t.Fatal(err)
	}
}

// edit returns the about template with a different sentence.
func (d *shopDev) edit(sentence string) string {
	return strings.Replace(d.original, "A shop built with Gx.", sentence, 1)
}

// A markup edit swaps into the running app: the page shows it, and the app
// does not restart (REQ-DEV-02).
func TestREQ_DEV_02_MarkupSwap(t *testing.T) {
	d := startShopDev(t)
	first := info(t, d.addr)
	if first.PID == 0 || first.Swapped != 0 {
		t.Fatalf("info at the start = %+v", first)
	}

	d.write(d.edit("Edited text one."))
	if ev := waitReload(t, d.ch, 20*time.Second); !ev.Swap {
		t.Fatal("the reload event is not a swap")
	}
	if body := page(t, d.url); !strings.Contains(body, "Edited text one.") || strings.Contains(body, "A shop built with Gx.") {
		t.Fatalf("the page does not show the edit:\n%s", body)
	}
	now := info(t, d.addr)
	if now.PID != first.PID {
		t.Fatalf("the app restarted: pid %d, then %d", first.PID, now.PID)
	}
	if now.Swapped == 0 {
		t.Fatal("no function runs as interpreted code")
	}
	// The generated file follows the template, so the next build compiles
	// the same code.
	generated, err := os.ReadFile(filepath.Join(d.dir, "About_gx.go"))
	if err != nil || !strings.Contains(string(generated), "Edited text one.") {
		t.Fatalf("About_gx.go does not hold the edit: %v", err)
	}

	// Control flow and expressions with symbols that the table holds.
	d.write(strings.Replace(d.edit("Edited text two."), `<span id="lazy-slot">waiting</span>`,
		"<span id=\"lazy-slot\">waiting</span>\nfor i := range 3 {\n  <b>{i + len(\"ab\")}</b>\n}\nif gx.BasePath() == \"\" {\n  <i>no base</i>\n}", 1))
	if ev := waitReload(t, d.ch, 20*time.Second); !ev.Swap {
		t.Fatal("the second reload event is not a swap")
	}
	body := page(t, d.url)
	for _, want := range []string{"Edited text two.", "<b>2</b>", "<b>4</b>", "<i>no base</i>"} {
		if !strings.Contains(body, want) {
			t.Fatalf("the page lacks %s:\n%s", want, body)
		}
	}

	// A template with an error shows the overlay with no restart, and the
	// fix swaps.
	d.write(d.edit("{p.NoSuchProp}"))
	ev := waitEvent(t, d.ch, "overlay", 20*time.Second)
	if !strings.Contains(ev.data, "NoSuchProp") || !strings.Contains(ev.data, "About.gx") {
		t.Fatalf("overlay = %s", ev.data)
	}
	d.write(d.edit("Fixed text."))
	if ev := waitReload(t, d.ch, 20*time.Second); !ev.Swap {
		t.Fatal("the fix is not a swap")
	}
	if body := page(t, d.url); !strings.Contains(body, "Fixed text.") {
		t.Fatalf("the page does not show the fix:\n%s", body)
	}
	if now := info(t, d.addr); now.PID != first.PID {
		t.Fatal("the app restarted")
	}

	// A change of the props changes Go types: it takes the rebuild path.
	d.write("package shop\n\nimport cartroute \"github.com/alternayte/gx/examples/shop/cart/route\"\n\nprops {\n  Note string = \"\"\n}\n\n<p>Props changed{p.Note}.</p>\n<div on:load={cartroute.Lazy{}}></div>\n<span id=\"lazy-slot\">waiting</span>\n")
	if ev := waitReload(t, d.ch, 60*time.Second); ev.Swap {
		t.Fatal("a change of the props was a swap")
	}
	getText(t, d.url, "Props changed.", 30*time.Second)
	if after := info(t, d.addr); after.PID == first.PID {
		t.Fatal("the app did not restart")
	}
}

// A reference outside the symbol table takes the rebuild path, and the
// page then shows the change (REQ-DEV-04).
func TestREQ_DEV_04_ReferenceOutsideTheTableRebuilds(t *testing.T) {
	d := startShopDev(t)
	before := info(t, d.addr)
	// A name of an import that the table holds swaps: strconv is a direct
	// import of the package of the template.
	d.write(d.edit(`Listed {gx.TextValue(len("abc"))}.`))
	if ev := waitReload(t, d.ch, 20*time.Second); !ev.Swap {
		t.Fatal("a call of a listed function was not a swap")
	}
	getText(t, d.url, "Listed 3.", 10*time.Second)
	// gx.Fixtures is a generic type: the symbol table cannot hold it, so
	// the app refuses the swap.
	d.write(d.edit(`Count {len(gx.Fixtures[int]{"a": 1, "b": 2})}.`))
	if ev := waitReload(t, d.ch, 60*time.Second); ev.Swap {
		t.Fatal("a reference outside the table was a swap")
	}
	getText(t, d.url, "Count 2.", 30*time.Second)
	after := info(t, d.addr)
	if after.PID == before.PID {
		t.Fatal("the app did not restart")
	}
	if after.Swapped != 0 {
		t.Fatalf("the new app runs %d functions as interpreted code", after.Swapped)
	}
}

// TestNFR_01_MarkupSwapUnder150ms measures a markup edit from the save to
// the page with the new text, on the example shop (NFR-01).
func TestNFR_01_MarkupSwapUnder150ms(t *testing.T) {
	d := startShopDev(t)
	var took []time.Duration
	for i := 0; i < 20; i++ {
		marker := "Timed edit " + string(rune('a'+i)) + "."
		start := time.Now()
		d.write(d.edit(marker))
		if ev := waitReload(t, d.ch, 20*time.Second); !ev.Swap {
			t.Fatal("the edit was not a swap")
		}
		// The browser asks for the page after the event; the answer with
		// the new text is the end of the server part.
		if body := page(t, d.url); !strings.Contains(body, marker) {
			t.Fatalf("the page does not show %q", marker)
		}
		took = append(took, time.Since(start))
	}
	sort.Slice(took, func(i, j int) bool { return took[i] < took[j] })
	p95 := took[len(took)*95/100-1]
	t.Logf("NFR-01: save to new page: median %s, p95 %s, max %s", took[len(took)/2], p95, took[len(took)-1])
	if budget := testbudget.Budget(150*time.Millisecond, 8); p95 > budget {
		t.Fatalf("NFR-01: p95 of save to new page is %s, want under %s on this machine", p95, budget)
	}
}
