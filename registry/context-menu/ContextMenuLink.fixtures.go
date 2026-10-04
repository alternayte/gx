package contextmenu

import "github.com/alternayte/gx"

var ContextMenuLinkFixtures = gx.Fixtures[ContextMenuLinkProps]{"Link": {Href: gx.URL("/docs"), Children: gx.Text("Docs")}}
