package inputgroup

import "github.com/alternayte/gx"

var InputGroupFixtures = gx.Fixtures[InputGroupProps]{
	"LeadingText": {Children: gx.Frag(
		InputGroupAddon(InputGroupAddonProps{Children: InputGroupText(InputGroupTextProps{Children: gx.Text("$")})}),
		InputGroupInput(InputGroupInputProps{Placeholder: "0.00"}),
	)},
	"TrailingText": {Children: gx.Frag(
		InputGroupInput(InputGroupInputProps{Placeholder: "example.com"}),
		InputGroupAddon(InputGroupAddonProps{Align: InlineEnd, Children: InputGroupText(InputGroupTextProps{Children: gx.Text(".com")})}),
	)},
}
