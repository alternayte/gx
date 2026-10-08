// Command widgetapp is the widget app of the repo's e2e suite (REQ-ISL-10).
// It runs two servers on two origins: a Gx app with one widget, and a host
// that serves plain HTML pages and the element file of the widget. The host
// is not a Gx app.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"strconv"
	"sync/atomic"

	"github.com/alternayte/gx"
	"github.com/alternayte/gx/adapters/datastar"
	"github.com/alternayte/gx/internal/gxstyles"
	"github.com/alternayte/gx/internal/widgetelement"
)

// cartIn is the input of the cart widget. Its fields are the attributes of
// <acme-cart>. A generated route type has the same Pattern and Bind methods.
type cartIn struct {
	Currency string
	Compact  bool
}

func (cartIn) Pattern() string { return "GET /cart" }

func (in *cartIn) Bind(r *http.Request) error {
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

func (in *cartIn) Rules() gx.Rules {
	return gx.Rules{gx.Field(&in.Currency, gx.OneOf("EUR", "USD", "SEK"))}
}

type cartProps struct {
	Currency string
	Compact  bool
	Loads    int64
}

// cartView is the component of the widget. The note field has no value from
// the server, so a test can see that a morph keeps what the user typed.
func cartView(p cartProps) gx.Node {
	// The signals and the client attributes below have the form that the
	// compiler writes for a component with a signals block. The text of
	// the adapter is empty: this app has no adapter, and a widget reads
	// the data form only (SI-15).
	const base = "cart.Cart"
	qty := gx.ExprSignal(base, "", "qty")
	open := gx.ExprSignal(base, "", "open")
	return gx.El("section", gx.Attrs{
		{Key: "id", Value: "cart"}, {Key: "data-compact", Value: strconv.FormatBool(p.Compact)},
		{Key: "data-signals", Value: gx.SignalJSON(base, "", map[string]any{"qty": 1, "open": false})},
		{Key: "data-gx-instance", Value: base},
	},
		gx.El("input", gx.Attrs{{Key: "id", Value: "qty"}, {Key: "type", Value: "number"}, {Key: "aria-label", Value: "Quantity"},
			{Key: "data-bind", Value: gx.SignalName(base, "", "qty")}}),
		gx.El("span", gx.Attrs{{Key: "id", Value: "count"}, gx.Client("data-text", "", qty)}),
		gx.El("p", gx.Attrs{{Key: "id", Value: "many"}, gx.Client("data-show", "", gx.ExprOp(">", qty, gx.ExprValue(2)))}, gx.Text("Many")),
		gx.El("button", gx.Attrs{{Key: "id", Value: "more"}, gx.Client("data-on:click", "",
			gx.ExprOp("do", gx.ExprOp("++", gx.ExprPath(base, "", "qty"))))}, gx.Text("More")),
		gx.El("button", gx.Attrs{{Key: "id", Value: "toggle"},
			gx.Client("data-attr:aria-expanded", "", open),
			gx.Client("data-on:click", "", gx.ExprOp("do", gx.ExprOp("=", gx.ExprPath(base, "", "open"), gx.ExprOp("!", open))))}, gx.Text("Details")),
		gx.El("div", gx.Attrs{{Key: "id", Value: "panel"}, gx.Client("data-class:is-open", "", open)}, gx.Text("Panel")),
		gx.El("button", gx.Attrs{{Key: "id", Value: "add"}, gx.On("click", "POST", "/widgets/cart/add", base)}, gx.Text("Add")),
		gx.El("span", gx.Attrs{{Key: "id", Value: "total"}}, gx.Text("0")),
		gx.El("button", gx.Attrs{{Key: "id", Value: "go"}, gx.On("click", "POST", "/widgets/cart/go", base)}, gx.Text("Checkout")),
		gx.El("button", gx.Attrs{{Key: "id", Value: "fail"}, gx.On("click", "POST", "/widgets/cart/fail", base)}, gx.Text("Fail")),
		gx.El("p", gx.Attrs{{Key: "id", Value: "line"}}, gx.Text("2 items in "+p.Currency)),
		gx.El("p", gx.Attrs{{Key: "id", Value: "loads"}}, gx.Text(strconv.FormatInt(p.Loads, 10))),
		gx.El("input", gx.Attrs{{Key: "id", Value: "note"}, {Key: "aria-label", Value: "Note"}}),
		// The classes of this box are the class list of the widget in
		// the Tailwind test.
		gx.El("div", gx.Attrs{{Key: "id", Value: "box"}, {Key: "class", Value: "p-4 shadow-lg rounded-xl bg-primary dark:bg-foreground"}}, gx.Text("Box")),
	)
}

// addIn is the input of the add action. Qty comes from the signal of the
// widget instance that sent the request.
type addIn struct{ Qty int }

func (addIn) Pattern() string { return "POST /cart/add" }

func (in *addIn) Bind(r *http.Request) error {
	signals, err := gx.Signals(r)
	if err != nil {
		return err
	}
	return gx.BindSignal(signals, gx.Scope(r), "qty", &in.Qty)
}

func (in *addIn) Rules() gx.Rules { return gx.Rules{gx.Field(&in.Qty, gx.Max(99))} }

type goIn struct{}

func (goIn) Pattern() string           { return "POST /cart/go" }
func (*goIn) Bind(*http.Request) error { return nil }

type failIn struct{}

func (failIn) Pattern() string           { return "POST /cart/fail" }
func (*failIn) Bind(*http.Request) error { return nil }

// checkout is the target of the redirect of the go action.
type checkout struct{}

func (checkout) URL() string { return "/checkout?step=1" }

// stamp makes a second build of this program a different file, so a test
// can run a new build of the server.
var stamp = "1"

func main() {
	api := flag.String("api", "127.0.0.1:8080", "address of the Gx app")
	host := flag.String("host", "127.0.0.1:8081", "address of the host pages")
	styles := flag.String("styles", "", "root of an app whose widget stylesheet Tailwind builds")
	flag.Parse()

	var loads atomic.Int64
	widget := gx.Widget(func(c *gx.Ctx, in cartIn) (cartProps, error) {
		// SEK is the currency that the loader of this app refuses.
		if in.Currency == "SEK" {
			return cartProps{}, gx.Forbidden()
		}
		return cartProps{Currency: in.Currency, Compact: in.Compact, Loads: loads.Add(1)}, nil
	}, cartView).Tag("acme-cart")

	// The stylesheet of the widget: the Tailwind build of the app at
	// -styles, as gx build makes it, and three rules of this test app.
	sheet := []byte(":host{--brand:rgb(1, 2, 3);display:block}#line{color:var(--brand);margin:0}#loads{color:rgb(9, 9, 9)}")
	if *styles != "" {
		built, err := gxstyles.BuildWidgets(context.Background(), *styles)
		if err != nil {
			log.Fatal(err)
		}
		if len(built["acme-cart"]) == 0 {
			log.Fatal("no stylesheet for acme-cart in " + *styles)
		}
		sheet = append(built["acme-cart"], sheet...)
	}
	gx.SetWidgetStylesheets(map[string][]byte{"acme-cart": sheet})

	add := gx.Action(func(c *gx.Ctx, in addIn) error {
		// The total of the widget, and the quantity back at 1.
		if err := c.Patch(gx.El("span", gx.Attrs{{Key: "id", Value: "total"}}, gx.Text(strconv.Itoa(in.Qty*10)))); err != nil {
			return err
		}
		return c.SetSignals(struct {
			Qty int `json:"qty"`
		}{Qty: 1})
	})
	goTo := gx.Action(func(c *gx.Ctx, in goIn) error { return c.Redirect(checkout{}) })
	fail := gx.Action(func(c *gx.Ctx, in failIn) error { return errors.New("the stock service is down, build " + stamp) })

	app := gx.New(gx.Config{})
	app.Group("/widgets", gx.AllowOrigins("http://"+*host), gx.Collect(widget, add, goTo, fail))
	go func() { log.Fatal(http.ListenAndServe(*api, app)) }()

	element := func(server string) []byte {
		file, err := widgetelement.File(widgetelement.Config{
			Tag: "acme-cart", Attrs: []string{"currency", "compact"}, Server: server, Path: "/widgets/cart",
		})
		if err != nil {
			log.Fatal(err)
		}
		return file
	}
	pages := http.NewServeMux()
	script := func(body []byte) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
			_, _ = w.Write(body)
		}
	}
	pages.HandleFunc("GET /acme-cart.js", script(element("http://"+*api)))
	// No server listens on port 9 of the loopback address.
	pages.HandleFunc("GET /down/acme-cart.js", script(element("http://127.0.0.1:9")))
	// The log of a host page: the events of the widget, the changes of its
	// state attribute, and each refusal of the policy of the page.
	pages.HandleFunc("GET /log.js", script([]byte(`window.log = []
for (const name of ['gx-ready', 'gx-error', 'gx-navigate']) {
  document.addEventListener(name, (e) => window.log.push([name, e.target.tagName, e.detail ?? null]))
}
document.addEventListener('securitypolicyviolation', (e) => window.log.push(['csp', e.violatedDirective, e.blockedURI]))
new MutationObserver((records) => {
  for (const r of records) {
    if (r.attributeName === 'data-gx-state') window.log.push(['state', r.target.getAttribute('data-gx-state')])
  }
}).observe(document.documentElement, { attributes: true, subtree: true })
`)))
	// The Datastar of a host page that uses it for itself.
	pages.HandleFunc("GET /datastar.js", script(datastar.Adapter().Assets()["datastar.js"]))
	page := func(src, tag string, policy ...string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			if len(policy) > 0 {
				w.Header().Set("Content-Security-Policy", policy[0])
			}
			_, _ = w.Write([]byte(`<!doctype html><html lang="en"><head><meta charset="utf-8"><title>Host</title>
<script src="/log.js"></script>
<script type="module" src="` + src + `"></script></head>
<body><h1>Host page</h1>` + tag + `</body></html>`))
		}
	}
	// A host with a strict policy: scripts of the host and of the Gx
	// server only, requests to the Gx server only, and no eval, no inline
	// script and no stylesheet of a different origin.
	pages.HandleFunc("GET /strict", page("/acme-cart.js", `<acme-cart></acme-cart>`,
		"default-src 'none'; script-src 'self' http://"+*api+"; connect-src http://"+*api))
	// A host that uses Datastar for its own page, with a signal that has
	// the name of a signal of the widget, and two instances of the widget.
	pages.HandleFunc("GET /two", page("/acme-cart.js", `<script type="module" src="/datastar.js"></script>
<div data-signals="{cart: {Cart: {qty: 100}}}"><span id="hostqty" data-text="$cart.Cart.qty"></span></div>
<acme-cart id="first"></acme-cart><acme-cart id="second"></acme-cart>`))
	pages.HandleFunc("GET /{$}", page("/acme-cart.js", `<acme-cart currency="USD"><p id="fallback">Loading your cart</p></acme-cart>`))
	// A host with its own styles and its own value for a token of the
	// widget.
	pages.HandleFunc("GET /themed", page("/acme-cart.js", `<style>p { color: rgb(200, 0, 0); margin: 40px } acme-cart.brand { --brand: rgb(0, 0, 250) }</style>
<p id="hostline">Text of the host</p>
<acme-cart id="plain"></acme-cart><acme-cart id="branded" class="brand"></acme-cart>`))
	pages.HandleFunc("GET /tailwind", page("/acme-cart.js", `<acme-cart id="light"></acme-cart><acme-cart id="dark" class="dark"></acme-cart>
<div id="hostbox" class="p-4 shadow-lg">A box of the host with the same class names</div>`))
	pages.HandleFunc("GET /down", page("/down/acme-cart.js", `<acme-cart currency="USD"><p id="fallback">Loading your cart</p></acme-cart>`))
	pages.HandleFunc("GET /bad", page("/acme-cart.js", `<acme-cart currency="GBP"><p id="fallback">Loading your cart</p></acme-cart>`))
	pages.HandleFunc("GET /refused", page("/acme-cart.js", `<acme-cart currency="SEK"><p id="fallback">Loading your cart</p></acme-cart>`))
	log.Fatal(http.ListenAndServe(*host, pages))
}
