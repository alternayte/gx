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
	starlight "github.com/alternayte/gx/registry/starlight-shell"
)

func main() {
	content.Install()
	// The code frames take their rules from the content stylesheet; the
	// Starlight theme sets their colours.
	gx.SetStylesheet(append(gxstyles.CSS(), content.CodeCSS()...))
	app := gx.New(gx.Config{Adapter: datastar.Adapter()})
	notFound := func(c *gx.Ctx) gx.Node {
		return starlight.NotFound(starlight.NotFoundProps{
			Site:    docs.Site,
			Title:   "404",
			Message: "Page not found. Check the URL or try using the search bar.",
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
