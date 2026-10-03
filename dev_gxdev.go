//go:build gxdev

package gx

import "net/http"

// devRoutes registers the routes that exist only in a dev build
// (REQ-DEV-07).
func (a *App) devRoutes() {
	a.mux.Handle("GET /_gx/dev/info", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"dev":true}`))
	}))
}
