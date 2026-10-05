package contextmenu

import "github.com/alternayte/gx"

var ContextMenuTriggerFixtures = gx.Fixtures[ContextMenuTriggerProps]{
	"Default": {
		Id:       "demo-context",
		Class:    "flex h-[150px] w-[300px] items-center justify-center rounded-md border border-dashed border-border text-sm",
		Children: gx.Text("Right-click here"),
	},
	"Sub": {
		Id:       "demo-context-sub",
		Class:    "flex h-[150px] w-[300px] items-center justify-center rounded-md border border-dashed border-border text-sm",
		Children: gx.Text("Right-click for a sub-menu"),
	},
}
