//go:build gxdev

package main

import (
	"github.com/alternayte/gx"
	"github.com/alternayte/gx/examples/shop/gxdev_gallery"
	"github.com/alternayte/gx/examples/shop/gxdev_symbols"
)

// setupGallery installs the dev gallery fixtures (REQ-AI-03). The file is
// gxdev only; the production stub does nothing (SI-08).
func setupGallery() {
	gx.SetGallery(gxdev_gallery.Fixtures())
	// The symbol table lets gx dev swap a changed template into the running
	// app with no rebuild (REQ-DEV-02).
	gx.SetDevSymbols(gxdev_symbols.Packages())
}
