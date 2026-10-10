package gx

import (
	"net/http"
	"strings"
)

// pageLinks returns the Link header values of one page: its stylesheet and
// the scripts that the page uses, and no other file (NFR-04). A CDN can send
// them as early hints, and the app sends them itself with status 103 for the
// next request of the route. A page with a nonce names no script: a preload
// from a header has no nonce, and a strict policy refuses it (SI-11).
func (a *App) pageLinks(needs *runtimeNeeds) []string {
	var links []string
	if len(Stylesheet()) != 0 {
		links = append(links, "<"+BasePath()+"/_gx/app.css>; rel=preload; as=style")
	}
	if needs == nil || needs.nonce != "" {
		return links
	}
	var walk func(Node)
	walk = func(n Node) {
		switch t := n.(type) {
		case fragNode:
			for _, c := range t {
				walk(c)
			}
		case *elNode:
			if t.name != "script" {
				return
			}
			src := attrValue(t.attrs, "src")
			if !strings.HasPrefix(src, "/") || strings.HasPrefix(src, "//") {
				// An inline script, or a file of a different origin.
				return
			}
			if attrValue(t.attrs, "type") == "module" {
				links = append(links, "<"+src+">; rel=modulepreload")
			} else {
				links = append(links, "<"+src+">; rel=preload; as=script")
			}
		}
	}
	if needs.theme && needs.shell != nil {
		walk(themeRuntime())
	}
	for _, n := range a.scriptNodes(needs) {
		walk(n)
	}
	return links
}

// sendEarlyHints answers 103 with the files that the last page of the route
// named, before the loaders of this request run. Only HTTP/2 and later get
// it: an HTTP/1.1 client can read an interim answer as the answer.
func (a *App) sendEarlyHints(w http.ResponseWriter, r *http.Request) {
	if r.ProtoMajor < 2 || r.Method != http.MethodGet || r.Header.Get("Gx-Nav") != "" || isWidgetRequest(r) {
		return
	}
	_, pattern := a.mux.Handler(r)
	links, ok := a.hints.Load(pattern)
	if !ok {
		return
	}
	w.Header()["Link"] = links.([]string)
	w.WriteHeader(http.StatusEarlyHints)
}
