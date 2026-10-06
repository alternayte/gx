//go:build !gxdev

package gx

import "net/http"

// Dev is false in a production build. Generated code asks it before it
// calls DevRender, so the compiler removes that call and the binary holds
// no part of the dev interpreter (SI-08).
const Dev = false

// DevRender is the dev hook of a generated function. A production build
// never calls it.
func DevRender(pkgPath, name string, args ...any) (Node, bool) { return nil, false }

// devRoutes is empty in a production build (REQ-DEV-07, SI-08).
func (a *App) devRoutes() {}

// exportRequest is false in a production build: only the export of a dev
// build renders a page for a static host (REQ-EXP-02).
func exportRequest(*http.Request) bool { return false }

// devKeepSignals is false in a production build: a page always sends the
// first values of its signals (REQ-DEV-03).
func devKeepSignals(*http.Request) bool { return false }
