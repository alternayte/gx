package gx

import (
	"net/http"
	"net/url"
	"strings"
	"sync"
)

// externals holds the actions that another server answers (REQ-EXP-02).
// Action.External fills it when the app declares its actions.
var externals struct {
	sync.RWMutex
	mux   *http.ServeMux
	bases map[string]string
}

// registerExternal records the base URL of the server that answers pattern.
func registerExternal(pattern, base string) {
	externals.Lock()
	defer externals.Unlock()
	if externals.mux == nil {
		externals.mux = http.NewServeMux()
		externals.bases = map[string]string{}
	}
	if _, ok := externals.bases[pattern]; !ok {
		externals.mux.Handle(pattern, http.NotFoundHandler())
	}
	externals.bases[pattern] = base
}

// externalURL returns the URL of an invocation on the server that answers
// it: the base URL plus the path, for an external action, or target
// unchanged.
func externalURL(method, target string) string {
	externals.RLock()
	defer externals.RUnlock()
	if externals.mux == nil {
		return target
	}
	path := strings.TrimPrefix(target, strings.TrimSuffix(BasePath(), "/"))
	u, err := url.Parse(path)
	if err != nil {
		return target
	}
	_, pattern := externals.mux.Handler(&http.Request{Method: method, URL: u})
	base, ok := externals.bases[pattern]
	if !ok {
		return target
	}
	return strings.TrimSuffix(base, "/") + path
}
