package sidebar

import "github.com/alternayte/gx"

var SidebarInputFixtures = gx.Fixtures[SidebarInputProps]{"Search": {Placeholder: "Search the docs", Attrs: gx.Attrs{{Key: "aria-label", Value: "Search"}}}}
