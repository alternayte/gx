package htmx_test

import (
	"encoding/json"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/alternayte/gx"
	"github.com/alternayte/gx/adapters/htmx"
)

// actRoute is a hand-written action input for the adapter contract tests.
type actRoute struct{}

func (actRoute) Pattern() string          { return "POST /act" }
func (actRoute) Bind(*http.Request) error { return nil }

// pageRoute is a hand-written page input.
type pageRoute struct{}

func (pageRoute) Pattern() string          { return "GET /page" }
func (pageRoute) Bind(*http.Request) error { return nil }

func serve(t *testing.T, h gx.Handler, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	app := gx.New(gx.Config{Adapter: htmx.Adapter()})
	app.Group("/", gx.Collect(h))
	if req.Method != http.MethodGet {
		req.Header.Set("HX-Request", "true")
	}
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	return rec
}

func count() gx.Node {
	return gx.El("span", gx.Attrs{{Key: "id", Value: "count"}}, gx.Text("2"))
}

// TestREQ_ACT_09_HtmxWire checks that the public hook reaches the wire
// format of htmx: HTML with one out-of-band swap for a patch, and the
// header that stops a swap into the element of the request.
func TestREQ_ACT_09_HtmxWire(t *testing.T) {
	h := gx.Action(func(c *gx.Ctx, in actRoute) error { return c.Patch(count()) })
	rec := serve(t, h, httptest.NewRequest("POST", "/act", nil))
	if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := rec.Header().Get("HX-Reswap"); got != "none" {
		t.Fatalf("HX-Reswap = %q, want none", got)
	}
	if got := rec.Header().Get("Gx-Answer"); got != "patches" {
		t.Fatalf("Gx-Answer = %q, want patches", got)
	}
	want := `<template><div hx-swap-oob="gx-morph:#count"><span id="count">2</span></div></template>`
	if got := rec.Body.String(); got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

// TestREQ_ACT_04_HtmxModes checks the swap style of each patch mode
// (REQ-ACT-04, contract test per adapter).
func TestREQ_ACT_04_HtmxModes(t *testing.T) {
	for _, tc := range []struct {
		name string
		arg  gx.Node
		want string
	}{
		{"default", nil, `<div hx-swap-oob="gx-morph:#count"><span id="count">2</span></div>`},
		{"append", gx.Append, `<div hx-swap-oob="beforeend:#count"><span id="count">2</span></div>`},
		{"prepend", gx.Prepend, `<div hx-swap-oob="afterbegin:#count"><span id="count">2</span></div>`},
		{"replace", gx.Replace, `<div hx-swap-oob="gx-replace:#count"><span id="count">2</span></div>`},
		{"remove", gx.Remove, `<div hx-swap-oob="delete:#count"></div>`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := gx.Action(func(ctx *gx.Ctx, in actRoute) error {
				if tc.arg == nil {
					return ctx.Patch(count())
				}
				return ctx.Patch(tc.arg, count())
			})
			rec := serve(t, h, httptest.NewRequest("POST", "/act", nil))
			if got, want := rec.Body.String(), "<template>"+tc.want+"</template>"; got != want {
				t.Fatalf("body = %s, want %s", got, want)
			}
		})
	}
}

// TestREQ_ACT_04_HtmxTableRow checks that a table row travels inside a
// tbody, so the HTML parser of the browser keeps it.
func TestREQ_ACT_04_HtmxTableRow(t *testing.T) {
	h := gx.Action(func(c *gx.Ctx, in actRoute) error {
		return c.Patch(gx.El("tr", gx.Attrs{{Key: "id", Value: "row"}}, gx.El("td", nil, gx.Text("1"))))
	})
	rec := serve(t, h, httptest.NewRequest("POST", "/act", nil))
	want := `<template><tbody hx-swap-oob="gx-morph:#row"><tr id="row"><td>1</td></tr></tbody></template>`
	if got := rec.Body.String(); got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

// TestREQ_ACT_09_HtmxAnswers checks a redirect, a toast, a view transition
// and a signal patch.
func TestREQ_ACT_09_HtmxAnswers(t *testing.T) {
	t.Run("redirect", func(t *testing.T) {
		h := gx.Action(func(c *gx.Ctx, in actRoute) error { return c.Redirect(gx.URL("/next")) })
		rec := serve(t, h, httptest.NewRequest("POST", "/act", nil))
		if got := rec.Header().Get("HX-Redirect"); got != "/next" {
			t.Fatalf("HX-Redirect = %q, want /next", got)
		}
	})
	t.Run("toast", func(t *testing.T) {
		h := gx.Action(func(c *gx.Ctx, in actRoute) error { return c.Toast("Saved") })
		body := serve(t, h, httptest.NewRequest("POST", "/act", nil)).Body.String()
		if !strings.HasPrefix(body, `<template><div hx-swap-oob="beforeend:#gx-toaster"><div role=`) || !strings.Contains(body, "Saved") {
			t.Fatalf("toast body = %s", body)
		}
	})
	t.Run("view transition", func(t *testing.T) {
		h := gx.Action(func(c *gx.Ctx, in actRoute) error { return c.Patch(gx.ViewTransition, count()) })
		rec := serve(t, h, httptest.NewRequest("POST", "/act", nil))
		if got := rec.Header().Get("HX-Reswap"); got != "none transition:true" {
			t.Fatalf("HX-Reswap = %q", got)
		}
	})
	t.Run("signals", func(t *testing.T) {
		h := gx.Action(func(c *gx.Ctx, in actRoute) error { return c.SetSignals(map[string]int{"qty": 2}) })
		rec := serve(t, h, httptest.NewRequest("POST", "/act", nil))
		if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "the adapter has no signals") {
			t.Fatalf("signal patch = %d %q, want a 500 that names signals", rec.Code, rec.Body.String())
		}
	})
}

// TestREQ_ACT_08_HtmxOn checks the attributes of an action invocation for
// each event modifier and each special event that htmx can express.
func TestREQ_ACT_08_HtmxOn(t *testing.T) {
	old := gx.AdapterOf(nil)
	defer gx.SetAdapter(old)
	gx.SetAdapter(htmx.Adapter())
	for _, tc := range []struct{ spec, method, want string }{
		{"click", "POST", `hx-post="/act" hx-trigger="click"`},
		{"click", "DELETE", `hx-delete="/act" hx-trigger="click"`},
		{"click.stop.once", "POST", `hx-post="/act" hx-trigger="click consume once"`},
		{"keydown.window", "POST", `hx-post="/act" hx-trigger="keydown from:window"`},
		{"input.debounce(300ms)", "POST", `hx-post="/act" hx-trigger="input delay:300ms"`},
		{"scroll.throttle(1s)", "POST", `hx-post="/act" hx-trigger="scroll throttle:1s"`},
		{"load", "GET", `hx-get="/act" hx-trigger="load"`},
		{"visible", "GET", `hx-get="/act" hx-trigger="intersect"`},
		{"interval(5s)", "GET", `hx-get="/act" hx-trigger="every 5s"`},
	} {
		got := gx.String(gx.El("i", gx.Attrs{gx.On(tc.spec, tc.method, "/act", "")}))
		if want := "<i " + tc.want + "></i>"; got != want {
			t.Errorf("on:%s = %s, want %s", tc.spec, got, want)
		}
	}
	// The compiler reports these as GX4006. A render writes no attribute.
	for _, spec := range []string{"click.outside", "click.prevent", "interval"} {
		if got := gx.String(gx.El("i", gx.Attrs{gx.On(spec, "POST", "/act", "")})); got != "<i></i>" {
			t.Errorf("on:%s = %s, want no attribute", spec, got)
		}
	}
}

// TestREQ_ACT_09_HtmxPage checks the scripts of a page with an action, and
// that a page with signals answers with the reason.
func TestREQ_ACT_09_HtmxPage(t *testing.T) {
	view := gx.El("button", gx.Attrs{gx.On("click", "POST", "/act", "")}, gx.Text("Add"))
	page := gx.Page(func(*gx.Ctx, pageRoute) (struct{}, error) { return struct{}{}, nil },
		func(struct{}) gx.Node { return view })
	body := serve(t, page, httptest.NewRequest("GET", "/page", nil)).Body.String()
	for _, want := range []string{
		`<script defer src="/_gx/htmx.js" data-gx-adapter="htmx"></script>`,
		`<script defer src="/_gx/idiomorph.js"></script>`,
		`<script defer src="/_gx/gx-htmx.js"></script>`,
		`<button hx-post="/act" hx-trigger="click">Add</button>`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("page lacks %s:\n%s", want, body)
		}
	}
	if strings.Contains(body, "datastar") {
		t.Fatalf("an htmx page names datastar:\n%s", body)
	}

	view = gx.El("div", gx.Attrs{{Key: "data-signals", Value: `{"qty":1}`}})
	rec := serve(t, page, httptest.NewRequest("GET", "/page", nil))
	if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), `Set adapter = "htmx" in gx.toml`) {
		t.Fatalf("page with signals = %d %q, want a 500 that names gx.toml", rec.Code, rec.Body.String())
	}
}

// TestREQ_ACT_09_HtmxAssets checks the pinned files (SI-10) and the Gx glue.
func TestREQ_ACT_09_HtmxAssets(t *testing.T) {
	a := htmx.Adapter()
	if a.Name() != "htmx" || a.Signals() {
		t.Fatalf("Name = %q, Signals = %v", a.Name(), a.Signals())
	}
	assets := a.Assets()
	for _, name := range []string{"htmx.js", "idiomorph.js", "gx-htmx.js"} {
		if len(assets[name]) == 0 {
			t.Fatalf("no asset %s", name)
		}
	}
	if !strings.Contains(string(assets["htmx.js"]), `version:"`+htmx.Version+`"`) {
		t.Fatalf("htmx.js is not version %s", htmx.Version)
	}
	glue := string(assets["gx-htmx.js"])
	for _, want := range []string{"allowEval = false", "includeIndicatorStyles = false", "Gx-CSRF", "gx-morph", "gx-inner", "gx-replace"} {
		if !strings.Contains(glue, want) {
			t.Fatalf("the glue lacks %q", want)
		}
	}
	if err := a.ReadSignals(httptest.NewRequest("POST", "/act", nil), &struct{}{}); err == nil {
		t.Fatal("ReadSignals: no error")
	}
}

// TestREQ_PLG_04_HtmxImportsOnlyPublicAPI checks that the adapter imports
// the public Gx API and the standard library only.
func TestREQ_PLG_04_HtmxImportsOnlyPublicAPI(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), e.Name(), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range file.Imports {
			path, _ := strconv.Unquote(imp.Path.Value)
			if path == "github.com/alternayte/gx" || !strings.Contains(path, ".") {
				continue
			}
			t.Errorf("%s imports %s", e.Name(), path)
		}
	}
}

// TestREQ_ISL_17_HtmxEvent checks the wire of a domain event: the HX-Trigger
// header, which htmx dispatches on the element of the request. A header
// value is ASCII.
func TestREQ_ISL_17_HtmxEvent(t *testing.T) {
	type detail struct {
		Count int    `json:"count"`
		Note  string `json:"note"`
	}
	changed := gx.Event[detail]("cart-changed")
	sent := gx.Event[detail]("composer-sent")
	h := gx.Action(func(c *gx.Ctx, in actRoute) error {
		c.Emit(changed(detail{Count: 3, Note: "wörld 日本 😀"}))
		c.Emit(sent(detail{Count: 1}))
		return nil
	})
	rec := serve(t, h, httptest.NewRequest("POST", "/act", nil))
	got := rec.Header().Get("HX-Trigger")
	for _, r := range got {
		if r >= 0x80 {
			t.Fatalf("HX-Trigger is not ASCII: %q", got)
		}
	}
	var events map[string]struct {
		Count int    `json:"count"`
		Note  string `json:"note"`
	}
	if err := json.Unmarshal([]byte(got), &events); err != nil {
		t.Fatalf("HX-Trigger is not JSON: %v: %s", err, got)
	}
	if len(events) != 2 || events["cart-changed"].Count != 3 || events["cart-changed"].Note != "wörld 日本 😀" || events["composer-sent"].Count != 1 {
		t.Fatalf("HX-Trigger = %s", got)
	}
	// An answer with no event has no such header.
	quiet := gx.Action(func(c *gx.Ctx, in actRoute) error { return c.Toast("Saved") })
	if got := serve(t, quiet, httptest.NewRequest("POST", "/act", nil)).Header().Get("HX-Trigger"); got != "" {
		t.Fatalf("HX-Trigger = %q for an answer with no event", got)
	}
}

// updatePair is what the compiler writes for two fragments:
//
//	<p #a>{a}</p><p #b>{b}</p>
var updatePair = gx.NewTemplate(
	[]string{"<p", ">", "</p><p", ">", "</p>"},
	[]int{0, 1, 0, 1},
	[]gx.TemplateEl{
		{Slot: 0, Start: 0, EndStatic: 2, End: len("</p>")},
		{Slot: 2, Start: len("</p>"), EndStatic: 4, End: len("</p>")},
	},
	[]gx.TemplateRoot{{El: 0, Slot: -1}, {El: 1, Slot: -1}},
)

func updateNode(a, b string) gx.Node {
	return updatePair.With(
		gx.OpenFragment("p", gx.Attrs{{Key: "id", Value: "pair-a"}}), gx.Text(a),
		gx.OpenFragment("p", gx.Attrs{{Key: "id", Value: "pair-b"}}), gx.Text(b),
	)
}

// TestREQ_ACT_16_UpdateContract checks c.Update on the wire of this adapter:
// the answer holds the fragment whose hash differs from the hash of the
// request, and a request with no hashes gets each fragment (REQ-ACT-16).
func TestREQ_ACT_16_UpdateContract(t *testing.T) {
	page := gx.StringRequest(httptest.NewRequest("GET", "/page", nil), updateNode("one", "two"))
	var have []string
	for _, m := range regexp.MustCompile(`id="([^"]+)" data-gx-h="([0-9a-f]{8})"`).FindAllStringSubmatch(page, -1) {
		have = append(have, m[1]+"="+m[2])
	}
	if len(have) != 2 {
		t.Fatalf("the fixture is wrong: the page has the hashes %v:\n%s", have, page)
	}
	h := gx.Action(func(c *gx.Ctx, in actRoute) error { return c.Update(updateNode("one", "three")) })

	req := httptest.NewRequest("POST", "/act", nil)
	req.Header.Set("Gx-Fragments", strings.Join(have, ","))
	body := serve(t, h, req).Body.String()
	if !strings.Contains(body, `id="pair-b"`) || !strings.Contains(body, "three") {
		t.Errorf("the answer lacks the changed fragment:\n%s", body)
	}
	if strings.Contains(body, `id="pair-a"`) {
		t.Errorf("the answer holds a fragment that did not change:\n%s", body)
	}

	body = serve(t, h, httptest.NewRequest("POST", "/act", nil)).Body.String()
	if !strings.Contains(body, `id="pair-a"`) || !strings.Contains(body, `id="pair-b"`) {
		t.Errorf("a request with no hashes did not get each fragment:\n%s", body)
	}
}
