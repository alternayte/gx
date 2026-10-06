//go:build gxdev

package gx

import (
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
)

// parentPipeEnv names the file descriptor that gx dev passes to the app. gx
// dev holds the other end of the pipe (internal/devserver).
const parentPipeEnv = "GX_DEV_PARENT_FD"

var watchParentOnce sync.Once

// watchParent ends the app when gx dev ends. A read on the pipe returns when
// gx dev closes its end, and the system closes that end when gx dev dies,
// also on a kill it cannot catch. An app started by hand has no pipe and
// runs on.
func watchParent() {
	fd, err := strconv.Atoi(os.Getenv(parentPipeEnv))
	if err != nil || fd < 3 {
		return
	}
	pipe := os.NewFile(uintptr(fd), "gx-dev-parent")
	if pipe == nil {
		return
	}
	go func() {
		_, _ = io.Copy(io.Discard, pipe)
		os.Exit(0)
	}()
}

// devRoutes registers the routes that exist only in a dev build
// (REQ-DEV-07).
func (a *App) devRoutes() {
	watchParentOnce.Do(watchParent)
	a.mux.Handle("GET /_gx/dev/info", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// The process id tells a test whether a change restarted the app;
		// swapped is the count of functions that run as interpreted code.
		_ = json.NewEncoder(w).Encode(map[string]any{"dev": true, "pid": os.Getpid(), "swapped": DevSwapped()})
	}))
	// gx dev sends the new generated code of a .gx file here. The answer
	// says whether the app runs it as interpreted code now, or needs a
	// rebuild (REQ-DEV-02).
	a.mux.Handle("POST /_gx/dev/swap", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Package string `json:"package"`
			File    string `json:"file"`
			Source  string `json:"source"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := DevSwap(in.Package, in.File, []byte(in.Source)); err != nil {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]any{"swapped": false, "reason": err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"swapped": true, "functions": DevSwapped()})
	}))
	a.mux.Handle("GET /_gx/gallery", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = RenderRequest(w, r, galleryPageHTML())
	}))
	a.mux.Handle("GET /_gx/gallery/fixture", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		f, ok := galleryFixture(q.Get("component"), q.Get("name"), q.Get("package"))
		if !ok {
			http.Error(w, "gx: no fixture "+q.Get("component")+" - "+q.Get("name"), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = RenderRequest(w, r, galleryFixtureHTML(f, q.Get("theme")))
	}))
	a.mux.Handle("GET /_gx/export", http.HandlerFunc(a.serveExportList))
}

// devReloadHeader marks the request that the dev client sends after a
// rebuild (REQ-DEV-03).
const devReloadHeader = "Gx-Dev-Reload"

// devKeepSignals reports whether the page renders for a dev reload. The
// first values of its signals then set only the signals that the browser
// does not hold, so the state of the page survives the rebuild.
func devKeepSignals(r *http.Request) bool {
	return r != nil && r.Header.Get(devReloadHeader) != ""
}

// exportHeader marks a request of `gx export` (REQ-EXP-02).
const exportHeader = "Gx-Export"

// exportRequest reports whether `gx export` asks for the page. The page
// then has no layout slot, so a link on a static host is a full load.
func exportRequest(r *http.Request) bool { return r.Header.Get(exportHeader) != "" }

// exportFeature is one mounted route that needs a server (REQ-EXP-02).
type exportFeature struct {
	Kind    string `json:"kind"`
	Pattern string `json:"pattern"`
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
		if _, ok := route.handler.(interface{ exportFeature() (string, bool) }); ok {
			// A GET action answers patches, not a page.
			continue
		}
		// "/{$}" is the exact form of a path that ends with a slash.
		pattern = strings.TrimSuffix(pattern, "{$}")
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
	assets = append(assets, islandFiles()...)
	if a.public != nil {
		// The app's own files export with the Gx assets (NFR-08).
		_ = fs.WalkDir(a.public, ".", func(name string, d fs.DirEntry, err error) error {
			if err == nil && d.Type().IsRegular() {
				assets = append(assets, "/"+name)
			}
			return nil
		})
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
	// An action that this app answers and a form need a server; the
	// export lists them and fails (REQ-EXP-02).
	serverOnly := []exportFeature{}
	for _, route := range a.routes {
		f, ok := route.handler.(interface{ exportFeature() (string, bool) })
		if !ok {
			continue
		}
		if kind, needsServer := f.exportFeature(); needsServer {
			serverOnly = append(serverOnly, exportFeature{Kind: kind, Pattern: route.pattern})
		}
	}
	out := struct {
		Paths      []string        `json:"paths"`
		Assets     []string        `json:"assets"`
		LLMS       LLMSManifest    `json:"llms"`
		ServerOnly []exportFeature `json:"serverOnly"`
	}{Paths: paths, Assets: assets, LLMS: llms, ServerOnly: serverOnly}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}
