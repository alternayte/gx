package popover

import "github.com/alternayte/gx"

var PopoverHeaderFixtures = gx.Fixtures[PopoverHeaderProps]{
	"Default": {Children: gx.Frag(
		PopoverTitle(PopoverTitleProps{Children: gx.Text("Dimensions")}),
		PopoverDescription(PopoverDescriptionProps{Children: gx.Text("Set the dimensions for the layer.")}),
	)},
}
