// Command docs is the Gx docs site (REQ-DOC-02): a Gx app that runs as a
// server in dev and exports to static files with `gx export`.
package main

import (
	"net/http"
	"os"

	"github.com/alternayte/gx"
	"github.com/alternayte/gx/adapters/datastar"
	"github.com/alternayte/gx/content"
	"github.com/alternayte/gx/docs/gxislands"
	"github.com/alternayte/gx/docs/gxstyles"
	"github.com/alternayte/gx/docs/site"
	"github.com/alternayte/gx/registry/docs-shell"
)

func main() {
	content.Install()
	// The code frames of the docs kit take their colours from the content
	// stylesheet; the Tailwind build does not hold them.
	gx.SetStylesheet(append(gxstyles.CSS(), content.CodeCSS()...))
	// The component pages show live fixtures; the tier 3 components are
	// islands (REQ-REG-14).
	gx.SetIslands(gxislands.Bundle())
	app := gx.New(gx.Config{Adapter: datastar.Adapter()})
	notFound := func(c *gx.Ctx) gx.Node {
		return shell.NotFound(shell.NotFoundProps{
			Title:   "Page not found",
			Message: "The page does not exist. Check the address or return to the start.",
			Home:    "/",
		})
	}
	app.Errors(notFound, notFound, notFound)
	app.Group("/", site.Routes)
	addr := os.Getenv("GX_DEV_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8082"
	}
	_ = http.ListenAndServe(addr, app)
}
