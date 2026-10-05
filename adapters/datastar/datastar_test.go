package datastar_test

import (
	"crypto/sha256"
	"encoding/hex"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/alternayte/gx"
	"github.com/alternayte/gx/adapters/datastar"
)

// actRoute is a hand-written action input for the adapter contract tests.
type actRoute struct{}

func (actRoute) Pattern() string          { return "POST /act" }
func (actRoute) Bind(*http.Request) error { return nil }

func serve(t *testing.T, h gx.Handler, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	app := gx.New(gx.Config{Adapter: datastar.Adapter()})
	app.Group("/", gx.Collect(h))
	req.Header.Set("Datastar-Request", "true")
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	return rec
}

// TestREQ_PLG_04_DatastarWire checks the public hook reaches the wire format
// of the pinned browser runtime.
func TestREQ_PLG_04_DatastarWire(t *testing.T) {
	h := gx.Action(func(c *gx.Ctx, in actRoute) error {
		return c.Patch(gx.El("span", gx.Attrs{{Key: "id", Value: "count"}}, gx.Text("2")))
	})
	rec := serve(t, h, httptest.NewRequest("POST", "/act", nil))
	body := rec.Body.String()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %q", rec.Code, body)
	}
	if !strings.Contains(body, "event: datastar-patch-elements") {
		t.Fatalf("SSE body lacks the element event:\n%s", body)
	}
	if !strings.Contains(body, "selector #count") {
		t.Fatalf("SSE body lacks the selector:\n%s", body)
	}
	if !strings.Contains(body, `<span id="count">2</span>`) {
		t.Fatalf("SSE body lacks the patch HTML:\n%s", body)
	}
	if strings.Contains(body, "mode append") {
		t.Fatalf("a morph patch must not set append mode:\n%s", body)
	}
}

// TestREQ_PLG_04_Invoke pins the client call of an action: the adapter owns
// the Datastar syntax, and the bytes are the ones the compiler wrote before.
func TestREQ_PLG_04_Invoke(t *testing.T) {
	a := datastar.Adapter()
	cases := []struct {
		method, scope, want string
	}{
		{"POST", "", `@post('/cart/add')`},
		{"POST", "cart.Cart.alpha", `@post('/cart/add', {headers: {'Gx-Scope': 'cart.Cart.alpha'}})`},
		{"GET", "", `@get('/cart/add')`},
		{"PUT", "", `@put('/cart/add')`},
		{"PATCH", "", `@patch('/cart/add')`},
		{"DELETE", "", `@delete('/cart/add')`},
	}
	for _, tt := range cases {
		got := a.Invoke(tt.method, "/cart/add", tt.scope)
		if got.Key != "data-on:click" || got.Value != tt.want {
			t.Fatalf("Invoke(%s, scope %q) = %s=%q, want data-on:click=%q", tt.method, tt.scope, got.Key, got.Value, tt.want)
		}
	}
	if got := a.Invoke("OPTIONS", "/cart/add", ""); got != (gx.Attr{}) {
		t.Fatalf("Invoke(OPTIONS) = %+v, want the zero Attr", got)
	}
}

// TestREQ_ACT_02_InvokeRendersTheSameBytes checks that an on: attribute
// built with gx.Invoke renders the bytes of the literal string it replaces.
func TestREQ_ACT_02_InvokeRendersTheSameBytes(t *testing.T) {
	old := gx.AdapterOf(nil)
	defer gx.SetAdapter(old)
	gx.SetAdapter(datastar.Adapter())
	scope := gx.ScopeString("cart.Cart", "alpha")
	button := func(value string) string {
		return gx.String(gx.El("button", gx.Attrs{{Key: "data-on:click", Value: value}}, gx.Text("Add")))
	}
	if got, want := button(gx.Invoke("POST", "/cart/add", scope).Value), button("@post('/cart/add', {headers: {'Gx-Scope': '"+scope+"'}})"); got != want {
		t.Fatalf("scoped action:\n got %s\nwant %s", got, want)
	}
	if got, want := button("$cart.qty = 2; "+gx.Invoke("GET", "/lazy", "").Value), button("$cart.qty = 2; @get('/lazy')"); got != want {
		t.Fatalf("action in a statement list:\n got %s\nwant %s", got, want)
	}
}

// TestREQ_PLG_04_SignalScope checks the invoking scope nests the signals.
func TestREQ_PLG_04_SignalScope(t *testing.T) {
	h := gx.Action(func(c *gx.Ctx, in actRoute) error {
		return c.SetSignals(map[string]int{"qty": 2})
	})
	req := httptest.NewRequest("POST", "/act", nil)
	req.Header.Set("Gx-Scope", "cart.42")
	rec := serve(t, h, req)
	body := rec.Body.String()
	if !strings.Contains(body, "event: datastar-patch-signals") {
		t.Fatalf("SSE body lacks the signal event:\n%s", body)
	}
	if !strings.Contains(body, `signals {"cart":{"42":{"qty":2}}}`) {
		t.Fatalf("SSE body lacks the nested signal scope:\n%s", body)
	}
}

// TestREQ_ACT_04_DatastarModes checks every patch mode reaches the wire
// (REQ-ACT-04, contract test per adapter).
func TestREQ_ACT_04_DatastarModes(t *testing.T) {
	cases := []struct {
		name string
		arg  gx.Node
		want string
	}{
		{"default", nil, ""},
		{"append", gx.Append, "mode append"},
		{"prepend", gx.Prepend, "mode prepend"},
		{"replace", gx.Replace, "mode replace"},
		{"remove", gx.Remove, "mode remove"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := gx.Action(func(ctx *gx.Ctx, in actRoute) error {
				node := gx.El("span", gx.Attrs{{Key: "id", Value: "count"}}, gx.Text("2"))
				if c.arg == nil {
					return ctx.Patch(node)
				}
				return ctx.Patch(c.arg, node)
			})
			rec := serve(t, h, httptest.NewRequest("POST", "/act", nil))
			body := rec.Body.String()
			if c.want == "" {
				if strings.Contains(body, "mode ") {
					t.Fatalf("default mode must morph:\n%s", body)
				}
				return
			}
			if !strings.Contains(body, c.want) {
				t.Fatalf("SSE body lacks %q:\n%s", c.want, body)
			}
		})
	}
}

// TestREQ_REG_11_DatastarToast checks the wire form of a toast: one append
// into the toaster region, rendered by Config.Toast of the app, or as the
// plain toast when the app sets none.
func TestREQ_REG_11_DatastarToast(t *testing.T) {
	h := gx.Action(func(c *gx.Ctx, in actRoute) error {
		return c.Toast("File uploaded", gx.ToastSuccess, gx.ToastID("upload"))
	})
	rec := serve(t, h, httptest.NewRequest("POST", "/act", nil))
	body := rec.Body.String()
	for _, want := range []string{
		"event: datastar-patch-elements",
		"selector #gx-toaster",
		"mode append",
		`<div id="gx-toast-upload" role="status" data-gx-toast="" data-kind="success" data-duration="4000">`,
		"File uploaded",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("SSE body lacks %q:\n%s", want, body)
		}
	}
	if n := strings.Count(body, "event: datastar-patch-elements"); n != 1 {
		t.Fatalf("toast sends %d element events, want 1:\n%s", n, body)
	}

	app := gx.New(gx.Config{Adapter: datastar.Adapter(), Toast: func(p gx.ToastPatch) gx.Node {
		return gx.El("output", gx.ToastAttrs(p), gx.Text("app: "+p.Text))
	}})
	app.Group("/", gx.Collect(h))
	req := httptest.NewRequest("POST", "/act", nil)
	req.Header.Set("Datastar-Request", "true")
	wired := httptest.NewRecorder()
	app.ServeHTTP(wired, req)
	if got := wired.Body.String(); !strings.Contains(got, `<output id="gx-toast-upload"`) || !strings.Contains(got, "app: File uploaded") {
		t.Fatalf("SSE body lacks the toast of Config.Toast:\n%s", got)
	}
}

// TestREQ_PLG_04_Assets checks the pinned runtime is served and matches its
// recorded hash (SI-10 groundwork).
func TestREQ_PLG_04_Assets(t *testing.T) {
	app := gx.New(gx.Config{Adapter: datastar.Adapter()})
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, httptest.NewRequest("GET", "/_gx/datastar.js", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("asset status = %d", rec.Code)
	}
	sum := sha256.Sum256(rec.Body.Bytes())
	if got := hex.EncodeToString(sum[:]); got != "727844adfc825ee651fb93c544a2a739986f9a21820a94524b35f0cac470cf91" {
		t.Fatalf("datastar.js hash = %s", got)
	}
	if !strings.Contains(rec.Body.String(), "Datastar v1.0.4") {
		t.Fatalf("asset is not Datastar v1.0.4")
	}
}

// TestREQ_PLG_04_ImportsOnlyPublicAPI checks an adapter uses public gx API
// only (REQ-PLG-04).
func TestREQ_PLG_04_ImportsOnlyPublicAPI(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	allowed := func(path string) bool {
		if path == "github.com/alternayte/gx" {
			return true
		}
		if strings.HasPrefix(path, "github.com/starfederation/datastar-go/") {
			return true
		}
		first := path
		if i := strings.IndexByte(path, '/'); i >= 0 {
			first = path[:i]
		}
		return !strings.Contains(first, ".")
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		src, err := os.ReadFile(e.Name())
		if err != nil {
			t.Fatal(err)
		}
		file, err := parser.ParseFile(token.NewFileSet(), e.Name(), src, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range file.Imports {
			path, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			if !allowed(path) {
				t.Errorf("%s imports %q; adapters use public API only", e.Name(), path)
			}
		}
	}
}
