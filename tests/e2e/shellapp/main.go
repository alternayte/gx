// Command shellapp is the docs-shell parity app of the repo's e2e suite
// (REQ-CNT-06). The suite copies it to a temp module and copies
// registry/docs-shell into ui/shell before generating.
package main

import (
	"net/http"
	"os"

	"github.com/alternayte/gx"
	"github.com/alternayte/gx/adapters/datastar"
	"github.com/alternayte/gx/content"
	"shellapp/ui/shell"
)

func main() {
	content.Install()
	app := gx.New(gx.Config{Adapter: datastar.Adapter()})
	app.Errors(
		func(c *gx.Ctx) gx.Node {
			return shell.NotFound(shell.NotFoundProps{Title: "Page not found", Message: "The page does not exist. Check the address or return to the start.", Home: "/"})
		},
		func(c *gx.Ctx) gx.Node {
			return shell.NotFound(shell.NotFoundProps{Title: "Forbidden", Message: "You cannot see this page.", Home: "/"})
		},
		func(c *gx.Ctx) gx.Node {
			return shell.NotFound(shell.NotFoundProps{Title: "Server error", Message: "The page failed.", Home: "/"})
		},
	)
	app.Group("/", Routes)
	_ = http.ListenAndServe("127.0.0.1:"+os.Getenv("PORT"), app)
}
