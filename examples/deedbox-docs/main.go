// Command deedbox-docs is the docs site example of the Gx repository: a
// port of the Deedbox docs (MIT), exported with `gx export` (REQ-CNT-14).
package main

import (
	"net/http"
	"os"

	"github.com/alternayte/gx"
	"github.com/alternayte/gx/adapters/datastar"
	"github.com/alternayte/gx/content"
	"github.com/alternayte/gx/examples/deedbox-docs/docs"
	"github.com/alternayte/gx/examples/deedbox-docs/gxstyles"
	"github.com/alternayte/gx/registry/docs-shell"
)

func main() {
	content.Install()
	gx.SetStylesheet(gxstyles.CSS())
	app := gx.New(gx.Config{Adapter: datastar.Adapter()})
	notFound := func(c *gx.Ctx) gx.Node {
		return shell.NotFound(shell.NotFoundProps{
			Title:   "Page not found",
			Message: "The page does not exist. Check the address or return to the start.",
			Home:    "/",
		})
	}
	app.Errors(notFound, notFound, notFound)
	app.Group("/", docs.Routes)
	addr := os.Getenv("GX_DEV_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8081"
	}
	_ = http.ListenAndServe(addr, app)
}
