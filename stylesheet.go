package gx

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"sync"
)

// stylesheet is the built Tailwind stylesheet of the app (REQ-STY-01 to
// REQ-STY-03). The scaffold's main installs it with gx.SetStylesheet; the
// app serves it at /_gx/app.css and links it in the page head.
var stylesheetState struct {
	mu  sync.RWMutex
	css []byte
}

// SetStylesheet installs the app stylesheet.
func SetStylesheet(css []byte) {
	stylesheetState.mu.Lock()
	defer stylesheetState.mu.Unlock()
	stylesheetState.css = append([]byte(nil), css...)
}

// Stylesheet returns the installed stylesheet, or nil.
func Stylesheet() []byte {
	stylesheetState.mu.RLock()
	defer stylesheetState.mu.RUnlock()
	return append([]byte(nil), stylesheetState.css...)
}

// stylesheetLink returns the link tag of the stylesheet of a page: the
// stylesheet of its route when the app has one, or the stylesheet of the
// app.
func stylesheetLink(pattern string) string {
	if url := stylesheetURL(pattern); url != "" {
		return `<link rel="stylesheet" href="` + url + `">`
	}
	return ""
}

// stylesheetURL returns the address of the stylesheet of a page route, or
// "" for an app with no stylesheet.
func stylesheetURL(pattern string) string {
	routeSheets.mu.RLock()
	name := routeSheets.byPattern[pattern]
	routeSheets.mu.RUnlock()
	if name != "" {
		return BasePath() + routeSheetPath + name
	}
	if len(Stylesheet()) == 0 {
		return ""
	}
	return BasePath() + "/_gx/app.css"
}

// routeSheetPath is the path of the stylesheets of the routes. The name of
// a file has the hash of its content, so a browser keeps it.
const routeSheetPath = "/_gx/css/"

// routeSheets holds the stylesheet of each page route that has its own
// (REQ-STY-13): the name of the file by the pattern of the route, and the
// content by the name.
var routeSheets struct {
	mu        sync.RWMutex
	byPattern map[string]string
	files     map[string][]byte
}

// SetRouteStylesheets installs the stylesheet of each page route, by the
// pattern of the route (REQ-STY-13). A generated app gives it
// gxstyles.Routes(). A page of such a route links its own stylesheet, which
// holds only the classes that the page can use; each other page links the
// stylesheet of the app. Two routes with the same content share one file.
func SetRouteStylesheets(sheets map[string][]byte) {
	byPattern, files := map[string]string{}, map[string][]byte{}
	for pattern, css := range sheets {
		if len(css) == 0 {
			continue
		}
		sum := sha256.Sum256(css)
		name := "app." + hex.EncodeToString(sum[:6]) + ".css"
		byPattern[pattern] = name
		files[name] = append([]byte(nil), css...)
	}
	routeSheets.mu.Lock()
	defer routeSheets.mu.Unlock()
	routeSheets.byPattern, routeSheets.files = byPattern, files
}

// hasRouteSheets reports whether a route of the app has its own stylesheet.
func hasRouteSheets() bool {
	routeSheets.mu.RLock()
	defer routeSheets.mu.RUnlock()
	return len(routeSheets.byPattern) > 0
}

// serveRouteSheet serves the stylesheet of a route by its name.
func serveRouteSheet(w http.ResponseWriter, r *http.Request) {
	routeSheets.mu.RLock()
	css, ok := routeSheets.files[r.PathValue("file")]
	routeSheets.mu.RUnlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	// The name has the hash of the content.
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	_, _ = w.Write(css)
}
