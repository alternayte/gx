package gx

import (
	"net/http"
	"strings"
	"sync"
)

// IslandBundle is the built JavaScript of the islands of an app
// (REQ-ISL-03). gx build writes it into the generated package gxislands,
// and the app's main installs it with gx.SetIslands(gxislands.Bundle()).
type IslandBundle struct {
	// Entries maps the name of an island to its file.
	Entries map[string]string
	// Files maps a file name to its content. Each name holds the hash of
	// the content.
	Files map[string]string
}

var islandState struct {
	mu sync.RWMutex
	b  IslandBundle
}

// SetIslands installs the island bundle of the app.
func SetIslands(b IslandBundle) {
	islandState.mu.Lock()
	defer islandState.mu.Unlock()
	islandState.b = b
}

// islandsPath is the URL path of the island files under the base path.
const islandsPath = "/_gx/islands/"

// islandSrc returns the URL of the entry file of an island, or "" when the
// installed bundle does not hold the island.
func islandSrc(name string) string {
	islandState.mu.RLock()
	defer islandState.mu.RUnlock()
	file, ok := islandState.b.Entries[name]
	if !ok {
		return ""
	}
	return BasePath() + islandsPath + file
}

// islandFiles returns the URL paths of the installed island files, for the
// static export.
func islandFiles() []string {
	islandState.mu.RLock()
	defer islandState.mu.RUnlock()
	out := make([]string, 0, len(islandState.b.Files))
	for _, name := range SortedKeys(islandState.b.Files) {
		out = append(out, islandsPath+name)
	}
	return out
}

// serveIsland serves one file of the island bundle. The name of a file
// holds the hash of its content, so a browser can keep it without end.
func serveIsland(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("file")
	islandState.mu.RLock()
	body, ok := islandState.b.Files[name]
	islandState.mu.RUnlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	contentType := "text/javascript; charset=utf-8"
	if strings.HasSuffix(name, ".map") {
		contentType = "application/json; charset=utf-8"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	_, _ = w.Write([]byte(body))
}
