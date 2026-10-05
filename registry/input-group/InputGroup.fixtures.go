package inputgroup

import (
	"github.com/alternayte/gx"
	"github.com/alternayte/gx/registry/button"
	"github.com/alternayte/gx/registry/icons"
)

// name returns the accessible name of a fixture control.
func name(label string) gx.Attrs {
	return gx.Attrs{{Key: "aria-label", Value: label}}
}

var InputGroupFixtures = gx.Fixtures[InputGroupProps]{
	"LeadingText": {Children: gx.Frag(
		InputGroupAddon(InputGroupAddonProps{Children: InputGroupText(InputGroupTextProps{Children: gx.Text("$")})}),
		InputGroupInput(InputGroupInputProps{Placeholder: "0.00", Attrs: name("Amount")}),
	)},
	"TrailingText": {Children: gx.Frag(
		InputGroupInput(InputGroupInputProps{Placeholder: "example", Attrs: name("Domain")}),
		InputGroupAddon(InputGroupAddonProps{Align: InlineEnd, Children: InputGroupText(InputGroupTextProps{Children: gx.Text(".com")})}),
	)},
	"Button": {Children: gx.Frag(
		InputGroupInput(InputGroupInputProps{Placeholder: "Search", Attrs: name("Search")}),
		InputGroupAddon(InputGroupAddonProps{Align: InlineEnd, Children: InputGroupButton(InputGroupButtonProps{Children: gx.Text("Search")})}),
	)},
	"IconButton": {Children: gx.Frag(
		InputGroupInput(InputGroupInputProps{Value: "ada@example.com", Attrs: name("Email")}),
		InputGroupAddon(InputGroupAddonProps{Align: InlineEnd, Children: InputGroupButton(InputGroupButtonProps{
			Size:     IconXs,
			Attrs:    name("Clear"),
			Children: icons.X(icons.XProps{}),
		})}),
	)},
	"Invalid": {Children: gx.Frag(
		InputGroupAddon(InputGroupAddonProps{Children: icons.Info(icons.InfoProps{})}),
		InputGroupInput(InputGroupInputProps{Value: "ada@", Attrs: gx.Attrs{{Key: "aria-label", Value: "Email"}, {Key: "aria-invalid", Value: "true"}}}),
	)},
	"Textarea": {Children: gx.Frag(
		InputGroupTextarea(InputGroupTextareaProps{Placeholder: "Ask a question.", Attrs: name("Question")}),
		InputGroupAddon(InputGroupAddonProps{Align: BlockEnd, Children: gx.Frag(
			InputGroupText(InputGroupTextProps{Children: gx.Text("120 characters left")}),
			InputGroupButton(InputGroupButtonProps{Variant: button.Default, Size: Sm, Class: "ml-auto", Children: gx.Text("Send")}),
		)}),
	)},
	"BlockStart": {Children: gx.Frag(
		InputGroupAddon(InputGroupAddonProps{Align: BlockStart, Children: InputGroupText(InputGroupTextProps{Children: gx.Text("Amount")})}),
		InputGroupInput(InputGroupInputProps{Placeholder: "0.00", Attrs: name("Amount")}),
	)},
}
