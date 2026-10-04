package scrollarea

import "github.com/alternayte/gx"

var ScrollAreaFixtures = gx.Fixtures[ScrollAreaProps]{
	"Vertical": {Class: "h-24 w-48 rounded-md border border-border p-2", Children: gx.Text("Line one. Line two. Line three. Line four. Line five. Line six. Line seven. Line eight.")},
}
