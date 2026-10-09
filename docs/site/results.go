package site

import (
	"embed"
	"regexp"
	"sort"
	"strings"

	"github.com/alternayte/gx"
	"github.com/alternayte/gx/docs/site/route"
)

// The result pages of the guides. `just docs-results` captures each one from
// the sample app of its guide: the content of the body with no script, and
// the stylesheet of the app. The docs check fails when a capture is stale.
//
//go:embed results
var resultFiles embed.FS

//go:embed result.js
var resultJS string

// result is one captured page.
type result struct {
	Get, Title, HTMLClass, BodyClass string
	Body, CSS                        string
}

var resultHeader = regexp.MustCompile(`^<!--gx-result get="([^"]*)" title="([^"]*)" html="([^"]*)" body="([^"]*)"-->\n`)

// resultLive finds the start of an attribute of the Gx runtime or of an
// adapter.
var resultLive = regexp.MustCompile(` (?:data-)?(gx-|on|signals|bind|init|hx-)`)

// resultKey returns the name of the capture of one request of one page:
// "tutorial/a-page" and "/shop/2" give "tutorial-a-page--shop-2".
func resultKey(slug, path string) string {
	clean := func(s string) string {
		var b strings.Builder
		for _, r := range strings.Trim(s, "/") {
			switch {
			case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
				b.WriteRune(r)
			default:
				b.WriteByte('-')
			}
		}
		return b.String()
	}
	return strings.TrimSuffix(clean(slug)+"--"+clean(path), "--")
}

// findResult reads one capture.
func findResult(key string) (result, bool) {
	if key == "" || strings.ContainsAny(key, "/.") {
		return result{}, false
	}
	html, err := resultFiles.ReadFile("results/" + key + ".html")
	if err != nil {
		return result{}, false
	}
	m := resultHeader.FindSubmatch(html)
	if m == nil {
		return result{}, false
	}
	css, _ := resultFiles.ReadFile("results/" + key + ".css")
	// The capture has no server and no script. Its form, action and signal
	// attributes get a different name, so no runtime and no export check
	// reads them as live features.
	body := resultLive.ReplaceAllStringFunc(string(html[len(m[0]):]), func(attr string) string {
		// Only an attribute of a runtime: "data-gx-", "data-on", "hx-".
		if !strings.HasPrefix(attr, " data-") && !strings.HasPrefix(attr, " hx-") {
			return attr
		}
		return " data-capture-" + strings.TrimPrefix(strings.TrimPrefix(attr, " "), "data-")
	})
	return result{
		Get: string(m[1]), Title: string(m[2]), HTMLClass: string(m[3]), BodyClass: string(m[4]),
		Body: body, CSS: string(css),
	}, true
}

// resultKeys lists the captures.
func resultKeys() []string {
	entries, _ := resultFiles.ReadDir("results")
	var out []string
	for _, e := range entries {
		if name, ok := strings.CutSuffix(e.Name(), ".html"); ok {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// resultNoteStyle is the look of the note of a result page. The page has
// the stylesheet of its sample app, not the one of the docs site.
const resultNoteStyle = "position:fixed;left:.5rem;right:.5rem;bottom:.5rem;z-index:50;margin:0;padding:.5rem .75rem;border:1px solid #8884;border-radius:.375rem;background:Canvas;color:CanvasText;font:12px/1.4 system-ui,sans-serif;box-shadow:0 2px 8px #0003"

// ResultPage renders one captured page as a document of its own.
var ResultPage = gx.Page(
	func(c *gx.Ctx, in route.Result) (result, error) {
		r, ok := findResult(in.Key)
		if !ok {
			return result{}, gx.NotFound()
		}
		return r, nil
	},
	func(r result) gx.Node {
		text := func(key, value string) gx.Attr { return gx.Attr{Key: key, Value: value, Kind: gx.AttrText} }
		return gx.El("html", gx.Attrs{text("lang", "en"), text("class", r.HTMLClass)},
			gx.El("head", nil,
				gx.El("meta", gx.Attrs{text("charset", "utf-8")}),
				gx.El("meta", gx.Attrs{text("name", "viewport"), text("content", "width=device-width, initial-scale=1")}),
				gx.El("meta", gx.Attrs{text("name", "robots"), text("content", "noindex")}),
				gx.El("title", nil, gx.Text(r.Title)),
				gx.El("style", nil, gx.Raw(gx.SafeHTML(r.CSS))), //gx:trusted the stylesheet is a capture in this package
			),
			gx.El("body", gx.Attrs{text("class", r.BodyClass)},
				gx.Raw(gx.SafeHTML(r.Body)), //gx:trusted the body is a capture in this package
				gx.El("p", gx.Attrs{text("id", "gx-result-note"), text("role", "status"), gx.Bool("hidden", true), {Key: "style", Value: resultNoteStyle, Kind: gx.AttrStyle}}),
				inlineScript(resultJS),
			),
		)
	},
).Static(func() ([]route.Result, error) {
	var out []route.Result
	for _, key := range resultKeys() {
		out = append(out, route.Result{Key: key})
	}
	return out, nil
})

// resultFor returns the route and the title of the capture that a guide
// shows.
func resultFor(page, get string) (route.Result, string) {
	key := resultKey(page, get)
	r, _ := findResult(key)
	return route.Result{Key: key}, r.Title
}

// resultTitle returns the title of the captured page of a guide.
func resultTitle(page, get string) string {
	_, title := resultFor(page, get)
	return title
}

// resultRoute returns the route of the captured page of a guide.
func resultRoute(page, get string) route.Result {
	r, _ := resultFor(page, get)
	return r
}
