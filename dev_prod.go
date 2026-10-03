//go:build !gxdev

package gx

// devRoutes is empty in a production build (REQ-DEV-07, SI-08).
func (a *App) devRoutes() {}
