package popover

import "github.com/alternayte/gx"

var PopoverTriggerFixtures = gx.Fixtures[PopoverTriggerProps]{
	"Default": {Id: "demo-popover", Children: gx.Text("Open popover")},
}
