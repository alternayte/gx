// Command adapterapp is the adapter contract app of the repo's e2e suite
// (REQ-ACT-09). The suite copies it to a temp module, generates it once and
// runs the one build under each adapter: ADAPTER=datastar or ADAPTER=htmx.
package main

import (
	"log"
	"net/http"
	"os"

	"adapterapp/board"

	"github.com/alternayte/gx"
	"github.com/alternayte/gx/adapters/datastar"
	"github.com/alternayte/gx/adapters/htmx"
)

func main() {
	// Datastar evaluates its expressions at runtime, so its policy needs
	// 'unsafe-eval'. The htmx adapter runs under the strict policy (SI-11).
	adapter, policy := datastar.Adapter(), gx.CSPOptions{UnsafeEval: true}
	if os.Getenv("ADAPTER") == "htmx" {
		adapter, policy = htmx.Adapter(), gx.CSPOptions{}
	}
	app := gx.New(gx.Config{Adapter: adapter})
	app.Group("/", board.Layout, gx.Nav(gx.MorphNavigation), board.Routes)
	log.Fatal(http.ListenAndServe("127.0.0.1:"+os.Getenv("PORT"), gx.CSP(policy)(app)))
}
