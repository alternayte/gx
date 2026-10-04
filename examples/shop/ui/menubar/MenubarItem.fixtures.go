package menubar

import "github.com/alternayte/gx"

var MenubarItemFixtures = gx.Fixtures[MenubarItemProps]{"Item": {Href: gx.URL("/docs"), Children: gx.Text("Docs")}}
