package dropdownmenu

import "github.com/alternayte/gx"

var DropdownMenuLinkFixtures = gx.Fixtures[DropdownMenuLinkProps]{"Link": {Href: gx.URL("/docs"), Children: gx.Text("Documentation")}}
