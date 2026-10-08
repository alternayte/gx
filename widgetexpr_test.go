package gx_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alternayte/gx"
)

func widgetRequest(method, target, body string) *http.Request {
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, target, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	} else {
		r = httptest.NewRequest(method, target, nil)
	}
	r.Header.Set("Gx-Widget", "acme-cart")
	return r
}

// TestSI_15_ExprTree checks the tree nodes of a client expression: each is
// JSON, and a server value is data in it.
func TestSI_15_ExprTree(t *testing.T) {
	sig := gx.ExprSignal("cart.Cart", gx.Key("42"), "qty")
	cases := []struct{ got, want string }{
		{gx.ExprValue(3), `["v",3]`},
		{gx.ExprValue("a\"b"), `["v","a\"b"]`},
		// A value cannot end a script or the attribute.
		{gx.ExprValue("</script><b>"), `["v","\u003c/script\u003e\u003cb\u003e"]`},
		{sig, `["s",["cart","Cart","42","qty"]]`},
		{gx.ExprSignal("cart.Cart", "", "qty"), `["s",["cart","Cart","qty"]]`},
		{gx.ExprPath("cart.Cart", gx.Key("42"), "qty"), `["cart","Cart","42","qty"]`},
		// A reference prop names the signal of a parent by its path.
		{gx.ExprRef(gx.SignalRefPath("page.Page", gx.Key("k\"1"), "open")), `["s",["page","Page","k\"1","open"]]`},
		{gx.ExprRefPath(gx.SignalRefPath("page.Page", "", "open")), `["page","Page","open"]`},
		{gx.ExprOp(">", sig, gx.ExprValue(3)), `[">",["s",["cart","Cart","42","qty"]],["v",3]]`},
		{gx.ExprOp("do", gx.ExprOp("++", gx.ExprPath("cart.Cart", "", "qty"))), `["do",["++",["cart","Cart","qty"]]]`},
		{gx.ExprCall("POST", "/cart/add", "cart.Cart.42"), `["call","POST","/cart/add","cart.Cart.42"]`},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("got  %s\nwant %s", tc.got, tc.want)
		}
		var any any
		if err := json.Unmarshal([]byte(tc.got), &any); err != nil {
			t.Errorf("%s is not JSON: %v", tc.got, err)
		}
	}
}

// TestSI_15_ClientAttributeForms checks that a client attribute renders as
// the text of the adapter for a page, and as data for a widget. The render
// for a widget holds no code.
func TestSI_15_ClientAttributeForms(t *testing.T) {
	tree := gx.ExprOp(">", gx.ExprSignal("cart.Cart", "", "qty"), gx.ExprValue(3))
	node := gx.El("div", gx.Attrs{
		{Key: "data-signals", Value: gx.SignalJSON("cart.Cart", "", map[string]any{"qty": 1})},
		{Key: "data-gx-instance", Value: "cart.Cart"},
	},
		gx.El("input", gx.Attrs{{Key: "data-bind", Value: gx.SignalName("cart.Cart", "", "qty")}}),
		gx.El("p", gx.Attrs{gx.Client("data-show", "($[\"cart\"][\"Cart\"][\"qty\"] > 3)", tree)}, gx.Text("Many")),
		gx.El("p", gx.Attrs{gx.Client("data-class:ring-2", "$open", gx.ExprSignal("cart.Cart", "", "open"))}),
		gx.El("button", gx.Attrs{gx.Client("data-on:click__prevent", "$x++; "+gx.Invoke("POST", "/save", "cart.Cart").Value,
			gx.ExprOp("do", gx.ExprOp("++", gx.ExprPath("cart.Cart", "", "x")), gx.ExprCall("POST", "/save", "cart.Cart")))}),
		gx.El("button", gx.Attrs{gx.On("click.once.debounce(300ms)", "POST", "/cart/add", "cart.Cart")}),
		gx.El("section", gx.Attrs{gx.On("visible", "GET", "/lazy", "")}),
		gx.El("section", gx.Attrs{gx.On("interval(5s)", "GET", "/poll", "")}),
		gx.El("section", gx.Attrs{gx.On("load", "GET", "/first", "")}),
		gx.El("button", gx.Attrs{gx.Invoke("POST", "/dismiss", "")}),
	)

	old := gx.AdapterOf(nil)
	defer gx.SetAdapter(old)
	gx.SetAdapter(&fakeAdapter{})
	page := httptest.NewRequest("GET", "/", nil)
	got := gx.StringRequest(page, node)
	for _, want := range []string{
		`data-signals="{&#34;cart&#34;:{&#34;Cart&#34;:{&#34;qty&#34;:1}}}"`,
		`<input data-bind="cart.Cart.qty">`,
		`<p data-show="($[&#34;cart&#34;][&#34;Cart&#34;][&#34;qty&#34;] &gt; 3)">`,
		`<p data-class:ring-2="$open">`,
		`<button data-on:click__prevent="$x++; fake(POST /save cart.Cart)">`,
		`<button data-fake-on-click="POST /cart/add">`,
		`<button data-fake-on="fake(POST /dismiss )">`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the page render has no %s\n%s", want, got)
		}
	}
	if strings.Contains(got, "data-gx-show") || strings.Contains(got, "data-gx-on") || strings.Contains(got, "&#34;call&#34;") {
		t.Errorf("the page render holds the widget form:\n%s", got)
	}

	widget := widgetRequest("GET", "/", "")
	got = gx.StringRequest(widget, node)
	q := func(s string) string { return strings.ReplaceAll(strings.ReplaceAll(s, `"`, "&#34;"), ">", "&gt;") }
	for _, want := range []string{
		`data-gx-signals="` + q(`{"cart":{"Cart":{"qty":1}}}`) + `"`,
		`<input data-gx-bind="cart.Cart.qty">`,
		`<p data-gx-show="` + q(`[">",["s",["cart","Cart","qty"]],["v",3]]`) + `">`,
		`<p data-gx-class:ring-2="` + q(`["s",["cart","Cart","open"]]`) + `">`,
		`<button data-gx-on:click__prevent="` + q(`["do",["++",["cart","Cart","x"]],["call","POST","/save","cart.Cart"]]`) + `">`,
		`<button data-gx-on:click__once__debounce.300ms="` + q(`["do",["call","POST","/cart/add","cart.Cart"]]`) + `">`,
		`<section data-gx-on-intersect="` + q(`["do",["call","GET","/lazy",""]]`) + `">`,
		`<section data-gx-on-interval__duration.5s="` + q(`["do",["call","GET","/poll",""]]`) + `">`,
		`<section data-gx-init="` + q(`["do",["call","GET","/first",""]]`) + `">`,
		`<button data-gx-on:click="` + q(`["do",["call","POST","/dismiss",""]]`) + `">`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the widget render has no %s\n%s", want, got)
		}
	}
	// No text of an adapter, and so no code, is in the render for a widget.
	for _, bad := range []string{"fake(", "data-fake", "data-show", "data-on:", "data-signals", "data-bind", "$[", "$x", "$open"} {
		if strings.Contains(got, bad) {
			t.Errorf("the widget render holds %q:\n%s", bad, got)
		}
	}
}

type signalIn struct {
	Qty int
}

func (signalIn) Pattern() string { return "POST /act" }

func (in *signalIn) Bind(r *http.Request) error {
	signals, err := gx.Signals(r)
	if err != nil {
		return err
	}
	return gx.BindSignal(signals, gx.Scope(r), "qty", &in.Qty)
}

func (in *signalIn) Rules() gx.Rules {
	return gx.Rules{gx.Field(&in.Qty, gx.Max(9))}
}

// TestREQ_ISL_21_WidgetWire checks the wire form of an action for a widget:
// the signals come as JSON of the request, and the answer is a list of steps
// as JSON, with no call of the adapter of the app.
func TestREQ_ISL_21_WidgetWire(t *testing.T) {
	adapter := &fakeAdapter{}
	var got signalIn
	h := gx.Action(func(c *gx.Ctx, in signalIn) error {
		got = in
		if err := c.Patch(gx.El("span", gx.Attrs{{Key: "id", Value: "total"}}, gx.Text("<20>"))); err != nil {
			return err
		}
		if err := c.SetSignals(struct {
			Qty int `json:"qty"`
		}{Qty: in.Qty + 1}); err != nil {
			return err
		}
		return c.Redirect(redirectTarget{})
	})
	app := gx.New(gx.Config{Adapter: adapter})
	app.Group("/", gx.Collect(h))

	req := widgetRequest("POST", "/act", `{"signals":{"cart":{"Cart":{"42":{"qty":2}}}}}`)
	req.Header.Set("Gx-Scope", "cart.Cart.42")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != 200 || got.Qty != 2 {
		t.Fatalf("status %d, Qty %d, body %s; want 200 and the signal value 2", rec.Code, got.Qty, rec.Body.String())
	}
	if adapter.responded {
		t.Error("the adapter of the app wrote the answer of a widget")
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q", ct)
	}
	var answer struct {
		Build string `json:"build"`
		Ops   []struct {
			Op, Mode, Target, HTML, Scope, URL string
			Values                             map[string]any
		} `json:"ops"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &answer); err != nil {
		t.Fatalf("the answer is not JSON: %v\n%s", err, rec.Body.String())
	}
	if len(answer.Build) != 16 || len(answer.Ops) != 3 {
		t.Fatalf("answer = %s", rec.Body.String())
	}
	patch, signals, redirect := answer.Ops[0], answer.Ops[1], answer.Ops[2]
	if patch.Op != "patch" || patch.Mode != "morph" || patch.Target != "#total" || patch.HTML != `<span id="total">&lt;20&gt;</span>` {
		t.Errorf("patch step = %+v", patch)
	}
	if signals.Op != "signals" || signals.Scope != "cart.Cart.42" || signals.Values["qty"] != float64(3) {
		t.Errorf("signals step = %+v", signals)
	}
	if redirect.Op != "redirect" || redirect.URL != "/home" {
		t.Errorf("redirect step = %+v", redirect)
	}

	// A signal value that fails a rule of the input is a field error for
	// the widget, and the handler does not run (DR-07).
	got = signalIn{}
	req = widgetRequest("POST", "/act", `{"signals":{"cart":{"Cart":{"42":{"qty":50}}}}}`)
	req.Header.Set("Gx-Scope", "cart.Cart.42")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	rec = httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != 422 || got.Qty != 0 || !strings.Contains(rec.Body.String(), `"error"`) || strings.Contains(rec.Body.String(), "<") {
		t.Errorf("tampered signal: status %d, handler Qty %d, body %s", rec.Code, got.Qty, rec.Body.String())
	}

	// A GET action reads the signals from the query.
	getIn := gx.Action(func(c *gx.Ctx, in signalGet) error { got.Qty = in.Qty; return nil })
	app2 := gx.New(gx.Config{Adapter: adapter})
	app2.Group("/", gx.Collect(getIn))
	req = widgetRequest("GET", "/poll?gx-signals=%7B%22cart%22%3A%7B%22qty%22%3A7%7D%7D", "")
	req.Header.Set("Gx-Scope", "cart")
	rec = httptest.NewRecorder()
	app2.ServeHTTP(rec, req)
	if rec.Code != 204 || got.Qty != 7 {
		t.Errorf("GET action: status %d, Qty %d; want 204 and 7", rec.Code, got.Qty)
	}
}

type signalGet struct{ Qty int }

func (signalGet) Pattern() string { return "GET /poll" }

func (in *signalGet) Bind(r *http.Request) error {
	signals, err := gx.Signals(r)
	if err != nil {
		return err
	}
	return gx.BindSignal(signals, gx.Scope(r), "qty", &in.Qty)
}
