package devserver

import (
	"encoding/json"
	"html"
	"net"
	"net/http"
	"strings"

	"github.com/alternayte/gx/internal/compiler"
	"github.com/alternayte/gx/internal/widgetelement"
)

// The dev host pages of the widgets (REQ-ISL-23). `gx dev` answers on the
// names localhost and 127.0.0.1. A browser sees two origins, so the page of
// a widget on one name loads the widget from the other name: the element,
// the token, the CORS answer and the cookie rule run as on a host of a
// different site. The app of a dev build accepts this one origin with no
// entry in gx.AllowOrigins.

// otherOrigin returns the origin of the same dev server under its other
// loopback name.
func otherOrigin(host string) string {
	name, port, err := net.SplitHostPort(host)
	if err != nil {
		return ""
	}
	other := "localhost"
	if name == "localhost" {
		other = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(other, port)
}

// serveWidgets serves the list of the widgets, the dev host page of one
// widget and its element file. Each other path below /_gx/widgets/ is a
// file of the app.
func (s *server) serveWidgets(app http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/_gx/widgets")
		name = strings.TrimPrefix(name, "/")
		tag, isScript := strings.CutSuffix(name, ".js")
		if strings.Contains(tag, ".") || strings.Contains(tag, "/") {
			// The stylesheet of a widget, from the app.
			app.ServeHTTP(w, r)
			return
		}
		widgets, diags := compiler.Widgets(s.dir)
		if len(diags) > 0 {
			http.Error(w, "gx dev: the app has diagnostics; fix them to see its widgets:\n"+diags[0].String(), http.StatusInternalServerError)
			return
		}
		if tag == "" {
			writeWidgetList(w, widgets)
			return
		}
		for _, wd := range widgets {
			if wd.Tag != tag {
				continue
			}
			if isScript {
				attrs := make([]string, len(wd.Attributes))
				for i, a := range wd.Attributes {
					attrs[i] = a.Name
				}
				file, err := widgetelement.File(widgetelement.Config{Tag: wd.Tag, Attrs: attrs, Server: otherOrigin(r.Host), Path: wd.Path})
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
				w.Header().Set("Cache-Control", "no-store")
				_, _ = w.Write(file)
				return
			}
			writeWidgetPage(w, r, wd)
			return
		}
		app.ServeHTTP(w, r)
	}
}

func writeWidgetList(w http.ResponseWriter, widgets []compiler.WidgetBuild) {
	var b strings.Builder
	b.WriteString(`<!doctype html><html lang="en"><head><meta charset="utf-8"><title>Widgets</title>` + widgetPageStyle + `</head><body><main><h1>Widgets</h1>`)
	if len(widgets) == 0 {
		b.WriteString(`<p>This app has no widget. Declare one with <code>gx.Widget(load, view).Tag("acme-name")</code>.</p>`)
	}
	b.WriteString(`<ul>`)
	for _, wd := range widgets {
		b.WriteString(`<li><a href="/_gx/widgets/` + html.EscapeString(wd.Tag) + `">` + html.EscapeString(wd.Tag) + `</a></li>`)
	}
	b.WriteString(`</ul></main></body></html>`)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(b.String()))
}

const widgetPageStyle = `<style>
body{font:15px/1.5 system-ui,sans-serif;margin:0;background:#fafafa;color:#111}
main{max-width:52rem;margin:0 auto;padding:1.5rem;display:grid;gap:1rem}
h1{font-size:1.25rem;margin:0}h2{font-size:1rem;margin:0}
fieldset{border:1px solid #ddd;border-radius:.5rem;padding:.75rem;display:grid;gap:.5rem}
label{display:flex;gap:.5rem;align-items:center}
input[type=text],input[type=number]{flex:1;font:inherit;padding:.25rem .5rem;border:1px solid #bbb;border-radius:.25rem}
.stage{border:1px dashed #bbb;border-radius:.5rem;padding:1rem;background:#fff}
ol{margin:0;padding-left:1.5rem;font:13px/1.5 ui-monospace,monospace}
code{font:13px ui-monospace,monospace}
</style>`

// writeWidgetPage writes the dev host page of one widget: the element, one
// field for each attribute, a field for the token, and a log of the events.
func writeWidgetPage(w http.ResponseWriter, r *http.Request, wd compiler.WidgetBuild) {
	tag := html.EscapeString(wd.Tag)
	events := []string{"gx-ready", "gx-error", "gx-navigate"}
	for _, e := range wd.Events {
		events = append(events, e.Name)
	}
	names, _ := json.Marshal(events)
	var b strings.Builder
	b.WriteString(`<!doctype html><html lang="en"><head><meta charset="utf-8"><title>Widget ` + tag + `</title>` + widgetPageStyle)
	b.WriteString(`<script type="module" src="/_gx/widgets/` + tag + `.js"></script></head><body><main>`)
	b.WriteString(`<h1>&lt;` + tag + `&gt;</h1>`)
	b.WriteString(`<p id="origins">This page is at <code>http://` + html.EscapeString(r.Host) + `</code>. The widget calls <code>` + html.EscapeString(otherOrigin(r.Host)) +
		`</code>, which is a different origin for the browser.</p>`)
	b.WriteString(`<fieldset id="attributes"><legend>Attributes</legend>`)
	if len(wd.Attributes) == 0 {
		b.WriteString(`<p>The widget has no attribute.</p>`)
	}
	for _, a := range wd.Attributes {
		name := html.EscapeString(a.Name)
		kind := "text"
		switch a.Type {
		case "number":
			kind = "number"
		case "boolean":
			kind = "checkbox"
		}
		b.WriteString(`<label>` + name + ` <input data-attr="` + name + `" type="` + kind + `" placeholder="` + html.EscapeString(a.Default) + `"></label>`)
	}
	b.WriteString(`</fieldset>`)
	b.WriteString(`<fieldset><legend>Token</legend><label>token <input id="token" type="text" placeholder="no token: the request has no Authorization header"></label></fieldset>`)
	b.WriteString(`<div class="stage"><` + tag + ` id="widget"><p>The widget loads.</p></` + tag + `></div>`)
	b.WriteString(`<h2>Events</h2><ol id="events"></ol>`)
	b.WriteString(`<script type="module">
const widget = document.getElementById('widget')
for (const input of document.querySelectorAll('[data-attr]')) {
  input.addEventListener('input', () => {
    const name = input.dataset.attr
    if (input.type === 'checkbox') widget.toggleAttribute(name, input.checked)
    else if (input.value === '') widget.removeAttribute(name)
    else widget.setAttribute(name, input.value)
  })
}
document.getElementById('token').addEventListener('change', (e) => {
  widget.token = e.target.value === '' ? undefined : e.target.value
})
const log = document.getElementById('events')
for (const name of ` + string(names) + `) {
  widget.addEventListener(name, (e) => {
    const item = document.createElement('li')
    item.textContent = name + ' ' + JSON.stringify(e.detail ?? null)
    log.append(item)
  })
}
</script></main></body></html>`)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(b.String()))
}
