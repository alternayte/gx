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
	return gx.El("section", gx.Attrs{{Key: "id", Value: "cart"}, {Key: "data-compact", Value: strconv.FormatBool(p.Compact)}},
		gx.El("p", gx.Attrs{{Key: "id", Value: "line"}}, gx.Text("2 items in "+p.Currency)),
		gx.El("p", gx.Attrs{{Key: "id", Value: "loads"}}, gx.Text(strconv.FormatInt(p.Loads, 10))),
		gx.El("input", gx.Attrs{{Key: "id", Value: "note"}, {Key: "aria-label", Value: "Note"}}),
		// The classes of this box are the class list of the widget in
		// the Tailwind test.
		gx.El("div", gx.Attrs{{Key: "id", Value: "box"}, {Key: "class", Value: "p-4 shadow-lg rounded-xl bg-primary dark:bg-foreground"}}, gx.Text("Box")),
	)
}

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

	app := gx.New(gx.Config{})
	app.Group("/widgets", gx.AllowOrigins("http://"+*host), gx.Collect(widget))
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
	page := func(src, tag string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<!doctype html><html lang="en"><head><meta charset="utf-8"><title>Host</title>
<script>
  window.log = []
  for (const name of ['gx-ready', 'gx-error']) {
    document.addEventListener(name, (e) => window.log.push([name, e.target.tagName, e.detail ?? null]))
  }
  new MutationObserver((records) => {
    for (const r of records) {
      if (r.attributeName === 'data-gx-state') window.log.push(['state', r.target.getAttribute('data-gx-state')])
    }
  }).observe(document.documentElement, { attributes: true, subtree: true })
</script>
<script type="module" src="` + src + `"></script></head>
<body><h1>Host page</h1>` + tag + `</body></html>`))
		}
	}
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
