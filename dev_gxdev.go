//go:build gxdev

package gx

import (
	"encoding/json"
	"net/http"
	"strings"
)

// devRoutes registers the routes that exist only in a dev build
// (REQ-DEV-07).
func (a *App) devRoutes() {
	a.mux.Handle("GET /_gx/dev/info", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"dev":true}`))
	}))
	a.mux.Handle("GET /_gx/gallery", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = RenderRequest(w, r, galleryPageHTML())
	}))
	a.mux.Handle("GET /_gx/export", http.HandlerFunc(a.serveExportList))
}

// serveExportList answers the dev-only export manifest: every GET page the
// static export renders, and the /_gx/ assets it copies (REQ-EXP-01).
func (a *App) serveExportList(w http.ResponseWriter, r *http.Request) {
	seen := map[string]bool{}
	var paths []string
	add := func(p string) {
		if p == "" || seen[p] {
			return
		}
		seen[p] = true
		paths = append(paths, p)
	}
	for _, route := range a.routes {
		method, pattern := "GET", route.pattern
		if m, p, ok := strings.Cut(route.pattern, " "); ok {
			method, pattern = m, p
		}
		if method != "GET" && method != "HEAD" {
			continue
		}
		if !strings.Contains(pattern, "{") {
			rel := strings.TrimPrefix(pattern, "/")
			base := strings.TrimSuffix(BasePath(), "/")
			add(base + "/" + rel)
			continue
		}
		ins, ok, err := StaticInputs(route.handler)
		if err != nil || !ok {
			continue
		}
		for _, in := range ins {
			if u, ok := in.(interface{ URL() string }); ok {
				add(u.URL())
			}
		}
	}
	assets := append([]string{}, a.assets...)
	if len(Stylesheet()) > 0 {
		assets = append(assets, "/_gx/app.css")
	}
	var llms LLMSManifest
	for _, route := range a.routes {
		src, ok := route.handler.(interface{ llmsManifest() LLMSManifest })
		if !ok {
			continue
		}
		m := src.llmsManifest()
		if llms.Site == "" {
			llms.Site = m.Site
		}
		if llms.Summary == "" {
			llms.Summary = m.Summary
		}
		llms.Entries = append(llms.Entries, m.Entries...)
	}
	out := struct {
		Paths  []string     `json:"paths"`
		Assets []string     `json:"assets"`
		LLMS   LLMSManifest `json:"llms"`
	}{Paths: paths, Assets: assets, LLMS: llms}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}
