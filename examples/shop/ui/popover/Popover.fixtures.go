package popover

import "github.com/alternayte/gx"

var PopoverFixtures = gx.Fixtures[PopoverProps]{
	"Content": {Id: "demo-popover", Children: PopoverHeader(PopoverHeaderProps{Children: gx.Frag(
		PopoverTitle(PopoverTitleProps{Children: gx.Text("Dimensions")}),
		PopoverDescription(PopoverDescriptionProps{Children: gx.Text("Set the dimensions for the layer.")}),
	)})},
	"Start": {Id: "demo-popover-start", Align: Start, Children: gx.Text("Place content for the popover here.")},
	"End":   {Id: "demo-popover-end", Align: End, Children: gx.Text("Place content for the popover here.")},
}
