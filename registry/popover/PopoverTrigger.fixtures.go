package popover

import (
	"github.com/alternayte/gx"
	"github.com/alternayte/gx/registry/button"
)

var PopoverTriggerFixtures = gx.Fixtures[PopoverTriggerProps]{
	"Default": {Id: "demo-popover", Children: gx.Text("Open popover")},
	"Start":   {Id: "demo-popover-start", Children: gx.Text("Open at the start")},
	"End":     {Id: "demo-popover-end", Variant: button.Secondary, Class: "ml-64", Children: gx.Text("Open at the end")},
}
