package sidebar

import "github.com/alternayte/gx"

var SidebarGroupFixtures = gx.Fixtures[SidebarGroupProps]{"Group": {Title: "Menu", Children: gx.Text("Items")}}
