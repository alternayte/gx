// Command shop is the reference Gx app. The repo's e2e suite drives it.
package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/alternayte/gx"
	"github.com/alternayte/gx/adapters/datastar"
	"github.com/alternayte/gx/examples/shop"
	"github.com/alternayte/gx/examples/shop/gxstyles"
	"github.com/alternayte/gx/examples/shop/ui/toast"
)

// listenAddr is the dev server address when gx dev runs the app.
func listenAddr() string {
	if v := os.Getenv("GX_DEV_ADDR"); v != "" {
		return v
	}
	return "127.0.0.1:8080"
}

func main() {
	addr := flag.String("addr", listenAddr(), "listen address")
	flag.Parse()
	setupGallery()
	gx.SetStylesheet(gxstyles.CSS())
	// The toast item renders every pushed toast, so its classes are in the
	// app stylesheet.
	app := gx.New(gx.Config{Adapter: datastar.Adapter(), Toast: toast.Render})
	app.Group("/", shop.ShellLayout, gx.Nav(gx.MorphNavigation), shop.Routes)
	log.Printf("shop listening on http://%s", *addr)
	log.Fatal(http.ListenAndServe(*addr, app))
}
