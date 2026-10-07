// Package htmx adapts Gx to the htmx hypermedia library through the public
// gx.Adapter hook (REQ-ACT-09, REQ-PLG-04). It imports only the public Gx
// API and the standard library.
//
// htmx has no client signals. The compiler reports a signal or a client
// expression of an htmx app as GX4006; the app sets adapter = "htmx" in its
// gx.toml.
package htmx

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"html"
	"net/http"
	"strings"

	"github.com/alternayte/gx"
)

// Version is the pinned htmx browser runtime version (SI-10).
const Version = "2.0.11"

// IdiomorphVersion is the pinned version of Idiomorph, the morph library of
// the htmx project. htmx has no morph of its own, and the default patch
// mode of Gx is a morph (REQ-ACT-04).
const IdiomorphVersion = "0.8.0"

// The SHA-256 of each pinned file. A mismatch stops the app (SI-10).
const (
	htmxSHA256      = "d6fdc75f204e6bdefa99b69bf1e6d4ac69b8a364f77929f45c13476b4000f717"
	idiomorphSHA256 = "4cbd535caf7663a51eda9bce6595371c384fc430d54b8d414e29a61167f19f96"
)

//go:embed htmx.js
var htmxJS []byte

//go:embed idiomorph.js
var idiomorphJS []byte

//go:embed gx-htmx.js
var glueJS []byte

type adapter struct{}

// Adapter returns the htmx adapter for gx.Config.Adapter.
func Adapter() gx.Adapter { return adapter{} }

func (adapter) Name() string  { return "htmx" }
func (adapter) Signals() bool { return false }

// Runtime returns the scripts every page needs (request lifecycle step 6):
// htmx, Idiomorph and the Gx glue, in this order.
func (adapter) Runtime() gx.Node {
	base := gx.BasePath()
	var b gx.Builder
	for _, name := range []string{"htmx.js", "idiomorph.js", "gx-htmx.js"} {
		attrs := gx.Attrs{
			gx.Bool("defer", true),
			{Key: "src", Value: base + "/_gx/" + name, Kind: gx.AttrURL},
		}
		if name == "htmx.js" {
			attrs = append(attrs, gx.Attr{Key: "data-gx-adapter", Value: "htmx"})
		}
		b.Add(gx.El("script", attrs))
	}
	return b.Node()
}

// Assets returns the browser runtime files served under /_gx/.
func (adapter) Assets() map[string][]byte {
	check("htmx.js", htmxJS, htmxSHA256)
	check("idiomorph.js", idiomorphJS, idiomorphSHA256)
	return map[string][]byte{
		"htmx.js":      htmxJS,
		"idiomorph.js": idiomorphJS,
		"gx-htmx.js":   glueJS,
	}
}

func check(name string, data []byte, want string) {
	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); got != want {
		panic("htmx: " + name + " hash mismatch: " + got)
	}
}

// errSignals is the answer to a signal read or a signal patch.
var errSignals = errors.New("htmx: the adapter has no signals (REQ-ACT-09)")

// Respond writes the answer as HTML with one out-of-band swap for each
// patch. The answer has no main content, so the HX-Reswap header tells htmx
// to swap nothing into the element that made the request. The Gx runtime
// reads the same answer for a form and for a navigation.
func (adapter) Respond(w http.ResponseWriter, r *http.Request, res *gx.Response) error {
	var b strings.Builder
	redirect := ""
	transition := false
	for _, p := range res.Patches {
		switch t := p.(type) {
		case gx.ElementPatch:
			content := ""
			if t.Mode != gx.ModeRemove {
				content = gx.StringRequest(r, t.Node)
			}
			writeSwap(&b, swapStyle(t.Mode), t.Target, content)
			transition = transition || t.Transition
		case gx.SignalPatch:
			return errSignals
		case gx.RedirectPatch:
			redirect = t.URL
		case gx.ToastPatch:
			// Append, so the toaster region keeps its id for the next
			// toast (REQ-REG-11). The behaviour runtime puts a toast with
			// an ID in the place of the earlier toast with that id.
			writeSwap(&b, "beforeend", "#gx-toaster", gx.StringRequest(r, gx.RenderToast(r, t)))
		}
	}
	if res.Navigate && res.Head != nil {
		// The Gx runtime reads this element and merges title, meta and
		// link tags (REQ-RTE-12).
		data, err := json.Marshal(res.Head)
		if err != nil {
			return err
		}
		b.WriteString(`<meta data-gx-head content="`)
		b.WriteString(html.EscapeString(string(data)))
		b.WriteString(`">`)
	}
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	h.Set("Gx-Answer", "patches")
	reswap := "none"
	if transition {
		// htmx runs the whole swap, with the out-of-band swaps, inside
		// startViewTransition (REQ-ACT-12).
		reswap = "none transition:true"
	}
	h.Set("HX-Reswap", reswap)
	if redirect != "" {
		h.Set("HX-Redirect", redirect)
	}
	if res.Status != 0 {
		w.WriteHeader(res.Status)
	}
	_, err := w.Write([]byte(b.String()))
	return err
}

// swapStyle returns the htmx swap style of a patch mode. The gx- styles are
// the styles of the Gx extension in gx-htmx.js.
func swapStyle(mode gx.PatchMode) string {
	switch mode {
	case gx.ModeInner:
		return "gx-inner"
	case gx.ModeAppend:
		return "beforeend"
	case gx.ModePrepend:
		return "afterbegin"
	case gx.ModeReplace:
		return "gx-replace"
	case gx.ModeRemove:
		return "delete"
	}
	return "gx-morph"
}

// writeSwap writes one out-of-band swap: an element that holds the content
// and names the style and the target selector. Each swap is in a template
// element of its own, so the HTML parser reads the content in the context
// of its holder: a table row needs a tbody around it.
func writeSwap(b *strings.Builder, style, target, content string) {
	holder := holderOf(content)
	b.WriteString("<template><")
	b.WriteString(holder)
	b.WriteString(` hx-swap-oob="`)
	b.WriteString(html.EscapeString(style + ":" + target))
	b.WriteString(`">`)
	b.WriteString(content)
	b.WriteString("</")
	b.WriteString(holder)
	b.WriteString("></template>")
}

// holderOf returns the element that can hold content as its children.
func holderOf(content string) string {
	name := ""
	if strings.HasPrefix(content, "<") {
		end := strings.IndexAny(content, " \t\n/>")
		if end > 0 {
			name = strings.ToLower(content[1:end])
		}
	}
	switch name {
	case "tr":
		return "tbody"
	case "td", "th":
		return "tr"
	case "thead", "tbody", "tfoot", "caption", "colgroup":
		return "table"
	case "col":
		return "colgroup"
	case "option", "optgroup":
		return "select"
	}
	return "div"
}

// Invoke returns the hx- attribute that calls the action with the HTTP
// method. htmx has no signals, so the scope does not travel.
func (adapter) Invoke(method, url, scope string) gx.Attr {
	switch method {
	case "GET", "POST", "PUT", "PATCH", "DELETE":
		return gx.Attr{Key: "hx-" + strings.ToLower(method), Value: url}
	}
	return gx.Attr{}
}

// On returns the hx- attribute of the action and the hx-trigger attribute
// of the event (REQ-ACT-08). It returns nil for a modifier that htmx cannot
// express; the compiler reports such a modifier as GX4006.
func (a adapter) On(inv gx.Invocation) []gx.Attr {
	call := a.Invoke(inv.Method, inv.URL, inv.Scope)
	if call.Key == "" {
		return nil
	}
	trigger := inv.Event
	switch inv.Event {
	case "visible":
		trigger = "intersect"
	case "interval":
		if inv.Every == "" {
			return nil
		}
		trigger = "every " + inv.Every
	}
	for _, mod := range inv.Mods {
		switch mod.Name {
		case "once":
			trigger += " once"
		case "stop":
			trigger += " consume"
		case "window":
			trigger += " from:window"
		case "debounce":
			trigger += " delay:" + mod.Value
		case "throttle":
			trigger += " throttle:" + mod.Value
		default:
			return nil
		}
	}
	return []gx.Attr{call, {Key: "hx-trigger", Value: trigger}}
}

// ReadSignals answers with an error: htmx has no signals.
func (adapter) ReadSignals(*http.Request, any) error { return errSignals }

var _ gx.Adapter = adapter{}
