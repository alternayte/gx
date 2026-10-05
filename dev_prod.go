//go:build !gxdev

package gx

import "net/http"

// devRoutes is empty in a production build (REQ-DEV-07, SI-08).
func (a *App) devRoutes() {}

// exportRequest is false in a production build: only the export of a dev
// build renders a page for a static host (REQ-EXP-02).
func exportRequest(*http.Request) bool { return false }
