// Command shop is the reference Gx app. The repo's e2e suite drives it.
package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/alternayte/gx"
	"github.com/alternayte/gx/adapters/datastar"
	"github.com/alternayte/gx/examples/shop"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "listen address")
	flag.Parse()
	app := gx.New(gx.Config{Adapter: datastar.Adapter()})
	app.Group("/", shop.ShellLayout, shop.Routes)
	log.Printf("shop listening on http://%s", *addr)
	log.Fatal(http.ListenAndServe(*addr, app))
}
