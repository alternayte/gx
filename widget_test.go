package gx_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/alternayte/gx"
)

// widgetIn is a hand-written widget input. A generated route type has the
// same Pattern and Bind methods.
type widgetIn struct {
	Currency string
	Compact  bool
}

func (widgetIn) Pattern() string { return "GET /cart" }

func (widgetIn) GxWidgetAttrs() []string { return []string{"currency", "compact"} }

func (in *widgetIn) Bind(r *http.Request) error {
	q := r.URL.Query()
	in.Currency = "EUR"
	if v := q.Get("currency"); v != "" {
		in.Currency = v
	}
	if v, ok := q["compact"]; ok {
		b, err := strconv.ParseBool(v[0])
		if err != nil {
			return errors.New("compact: not a bool")
		}
		in.Compact = b
	}
	return nil
}

func (in *widgetIn) Rules() gx.Rules {
	return gx.Rules{gx.Field(&in.Currency, gx.OneOf("EUR", "USD"))}
}

type cartProps struct {
	Currency string
	Compact  bool
	Items    int
}

func cartView(p cartProps) gx.Node {
	return gx.El("section", gx.Attrs{{Key: "data-compact", Value: strconv.FormatBool(p.Compact)}},
		gx.Text(strconv.Itoa(p.Items)+" items in "+p.Currency+" <&>"))
}

func cartWidget(load func(*gx.Ctx, widgetIn) (cartProps, error)) gx.Handler {
	if load == nil {
		load = func(c *gx.Ctx, in widgetIn) (cartProps, error) {
			return cartProps{Currency: in.Currency, Compact: in.Compact, Items: 2}, nil
		}
	}
	return gx.Widget(load, cartView).Tag("acme-cart")
}

// widgetAnswer is the first answer of a widget, as the element reads it.
type widgetAnswer struct {
	Tag   string `json:"tag"`
	HTML  string `json:"html"`
	Error *struct {
		Status int    `json:"status"`
		Key    string `json:"key"`
		Field  string `json:"field"`
	} `json:"error"`
}

func getWidget(t *testing.T, h gx.Handler, target string, mutate func(*http.Request)) (*httptest.ResponseRecorder, widgetAnswer) {
	t.Helper()
	app := gx.New(gx.Config{Adapter: &fakeAdapter{}})
	app.Group("/widgets", gx.AllowOrigins("https://shop.example.com"), gx.Collect(h))
	req := httptest.NewRequest("GET", "https://api.acme.dev"+target, nil)
	req.Header.Set("Origin", "https://shop.example.com")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	req.Header.Set("Gx-Widget", "acme-cart")
	if mutate != nil {
		mutate(req)
	}
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	var a widgetAnswer
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json; body %q", ct, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &a); err != nil {
		t.Fatalf("the answer is not JSON: %v\n%s", err, rec.Body.String())
	}
	return rec, a
}

// TestREQ_ISL_10_WidgetFirstAnswer checks that a widget route answers with
// the HTML of the component, as the server renders it, and with no document
// shell around it.
func TestREQ_ISL_10_WidgetFirstAnswer(t *testing.T) {
	rec, a := getWidget(t, cartWidget(nil), "/widgets/cart", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	want := gx.String(cartView(cartProps{Currency: "EUR", Items: 2}))
	if a.HTML != want {
		t.Errorf("html = %q, want the render of the component %q", a.HTML, want)
	}
	if a.Tag != "acme-cart" {
		t.Errorf("tag = %q", a.Tag)
	}
	if strings.Contains(rec.Body.String(), "<!doctype") || strings.Contains(rec.Body.String(), "<html") {
		t.Errorf("the answer has a document shell: %s", rec.Body.String())
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://shop.example.com" {
		t.Errorf("Access-Control-Allow-Origin = %q", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store: the answer is for one user", got)
	}
}

// TestREQ_ISL_10_WidgetLoaderError checks that an error of the loader gives
// its status and a message key, and no text of the server.
func TestREQ_ISL_10_WidgetLoaderError(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
		key    string
	}{
		{"not found", gx.NotFound(), 404, "gx.not_found"},
		{"forbidden", gx.Forbidden(), 403, "gx.forbidden"},
		{"an error of the app", errors.New("pq: password authentication failed for user admin"), 500, "gx.error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := cartWidget(func(c *gx.Ctx, in widgetIn) (cartProps, error) { return cartProps{}, tc.err })
			rec, a := getWidget(t, h, "/widgets/cart", nil)
			if rec.Code != tc.status || a.Error == nil || a.Error.Status != tc.status || a.Error.Key != tc.key {
				t.Fatalf("status = %d, answer %s; want %d and the key %s", rec.Code, rec.Body.String(), tc.status, tc.key)
			}
			if a.HTML != "" || strings.Contains(rec.Body.String(), "password") {
				t.Errorf("the answer holds HTML or text of the server: %s", rec.Body.String())
			}
		})
	}
}

// TestREQ_ISL_10_WidgetTag checks the tag rules: a widget needs a valid
// custom element name, and one app has each tag one time.
func TestREQ_ISL_10_WidgetTag(t *testing.T) {
	load := func(c *gx.Ctx, in widgetIn) (cartProps, error) { return cartProps{}, nil }
	panics := func(name string, fn func()) {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("no panic")
				}
			}()
			fn()
		})
	}
	for _, tag := range []string{"cart", "Acme-Cart", "1-cart", "acme cart", "-cart", "font-face", "annotation-xml", "", "acme-cart<"} {
		panics("tag "+strconv.Quote(tag), func() { gx.Widget(load, cartView).Tag(tag) })
	}
	panics("no tag", func() {
		gx.New(gx.Config{}).Group("/w", gx.Collect(gx.Widget(load, cartView)))
	})
	panics("one tag two times", func() {
		app := gx.New(gx.Config{})
		app.Group("/a", gx.Collect(gx.Widget(load, cartView).Tag("acme-cart")))
		app.Group("/b", gx.Collect(gx.Widget(load, cartView).Tag("acme-cart")))
	})
	for _, tag := range []string{"acme-cart", "x-y", "acme-cart-2", "my-élément", "a-b.c_d"} {
		gx.Widget(load, cartView).Tag(tag)
	}
}

// TestREQ_ISL_15_WidgetAttributes checks that the attributes of the element
// arrive as the query, go through the binder and the rules, and reach the
// component only through the loader.
func TestREQ_ISL_15_WidgetAttributes(t *testing.T) {
	t.Run("attributes fill the input", func(t *testing.T) {
		rec, a := getWidget(t, cartWidget(nil), "/widgets/cart?currency=USD&compact=true", nil)
		want := gx.String(cartView(cartProps{Currency: "USD", Compact: true, Items: 2}))
		if rec.Code != 200 || a.HTML != want {
			t.Fatalf("status %d, html %q, want %q", rec.Code, a.HTML, want)
		}
	})
	t.Run("a value that does not convert is status 400", func(t *testing.T) {
		rec, a := getWidget(t, cartWidget(nil), "/widgets/cart?compact=maybe", nil)
		if rec.Code != 400 || a.Error == nil || a.Error.Status != 400 || a.Error.Key != "gx.bad_attribute" {
			t.Fatalf("status %d, answer %s", rec.Code, rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), "not a bool") {
			t.Errorf("the answer holds text of the server: %s", rec.Body.String())
		}
	})
	t.Run("a value that fails a rule is status 400 with the field", func(t *testing.T) {
		ran := false
		h := cartWidget(func(c *gx.Ctx, in widgetIn) (cartProps, error) { ran = true; return cartProps{}, nil })
		rec, a := getWidget(t, h, "/widgets/cart?currency=GBP", nil)
		if rec.Code != 400 || a.Error == nil || a.Error.Key != "oneof" {
			t.Fatalf("status %d, answer %s; want 400 and the key of the rule", rec.Code, rec.Body.String())
		}
		if ran {
			t.Error("the loader ran with an input that failed a rule")
		}
	})
}

// TestREQ_ISL_10_ElementFileEndpoint checks that the app serves the element
// file of a widget to each origin, with no package: the file names the tag,
// the attributes and the path of the route, takes the origin of the server
// from its own URL, and has an ETag for a conditional request.
func TestREQ_ISL_10_ElementFileEndpoint(t *testing.T) {
	app := gx.New(gx.Config{Adapter: &fakeAdapter{}})
	app.Group("/widgets", gx.AllowOrigins("https://shop.example.com"), gx.Collect(cartWidget(nil)))
	get := func(target, etag string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", "https://api.acme.dev"+target, nil)
		if etag != "" {
			req.Header.Set("If-None-Match", etag)
		}
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, req)
		return rec
	}
	rec := get("/_gx/widgets/acme-cart.js", "")
	if rec.Code != 200 {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	h := rec.Header()
	if ct := h.Get("Content-Type"); !strings.HasPrefix(ct, "text/javascript") {
		t.Errorf("Content-Type = %q", ct)
	}
	if h.Get("Access-Control-Allow-Origin") != "*" || h.Get("Cache-Control") != "no-cache" {
		t.Errorf("headers = %v", h)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `{"tag":"acme-cart","attrs":["currency","compact"],"server":"","path":"/widgets/cart","self":true}`) {
		t.Errorf("the element file has no configuration of the widget:\n%.300s", body)
	}
	if !strings.Contains(body, "customElements.define") || !strings.Contains(body, "import.meta.url") {
		t.Errorf("the element file is not the loader, or does not read its own URL")
	}
	etag := h.Get("ETag")
	if etag == "" {
		t.Fatal("no ETag")
	}
	if again := get("/_gx/widgets/acme-cart.js", etag); again.Code != http.StatusNotModified || again.Body.Len() != 0 {
		t.Errorf("a request with the ETag: status %d, %d bytes", again.Code, again.Body.Len())
	}
	if missing := get("/_gx/widgets/acme-none.js", ""); missing.Code != http.StatusNotFound {
		t.Errorf("the element file of no widget: status %d", missing.Code)
	}
}

// TestREQ_ISL_10_WidgetInputNeedsAttrs checks that a widget input with no
// generated attribute list stops the app at its start.
func TestREQ_ISL_10_WidgetInputNeedsAttrs(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("no panic")
		}
	}()
	gx.Widget(func(c *gx.Ctx, in bareIn) (cartProps, error) { return cartProps{}, nil }, cartView)
}

// bareIn is a route input with the code of an older generate: it has no
// attribute list.
type bareIn struct{}

func (bareIn) Pattern() string           { return "GET /bare" }
func (*bareIn) Bind(*http.Request) error { return nil }
