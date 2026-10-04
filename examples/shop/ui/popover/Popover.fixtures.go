package popover

import "github.com/alternayte/gx"

var PopoverFixtures = gx.Fixtures[PopoverProps]{
	"Content": {Id: "demo-popover", Children: gx.Text("Place content for the popover here.")},
}
