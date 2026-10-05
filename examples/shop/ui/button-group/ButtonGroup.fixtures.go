package buttongroup

import (
	"github.com/alternayte/gx"
	"github.com/alternayte/gx/examples/shop/ui/button"
	"github.com/alternayte/gx/examples/shop/ui/icons"
)

var ButtonGroupFixtures = gx.Fixtures[ButtonGroupProps]{
	"Three": {Children: gx.Frag(
		button.Button(button.ButtonProps{Variant: button.Outline, Children: gx.Text("One")}),
		button.Button(button.ButtonProps{Variant: button.Outline, Children: gx.Text("Two")}),
		button.Button(button.ButtonProps{Variant: button.Outline, Children: gx.Text("Three")}),
	)},
	"Vertical": {Orientation: Vertical, Children: gx.Frag(
		button.Button(button.ButtonProps{Variant: button.Outline, Children: gx.Text("One")}),
		button.Button(button.ButtonProps{Variant: button.Outline, Children: gx.Text("Two")}),
		button.Button(button.ButtonProps{Variant: button.Outline, Children: gx.Text("Three")}),
	)},
	"Separator": {Children: gx.Frag(
		button.Button(button.ButtonProps{Variant: button.Secondary, Children: gx.Text("Copy")}),
		ButtonGroupSeparator(ButtonGroupSeparatorProps{}),
		button.Button(button.ButtonProps{Variant: button.Secondary, Size: button.Icon, Attrs: gx.Attrs{{Key: "aria-label", Value: "More"}}, Children: icons.ChevronDown(icons.ChevronDownProps{})}),
	)},
	"Text": {Children: gx.Frag(
		ButtonGroupText(ButtonGroupTextProps{Children: gx.Text("https://")}),
		button.Button(button.ButtonProps{Variant: button.Outline, Children: gx.Text("example.com")}),
	)},
	"Nested": {Children: gx.Frag(
		ButtonGroup(ButtonGroupProps{Children: gx.Frag(
			button.Button(button.ButtonProps{Variant: button.Outline, Children: gx.Text("One")}),
			button.Button(button.ButtonProps{Variant: button.Outline, Children: gx.Text("Two")}),
		)}),
		ButtonGroup(ButtonGroupProps{Children: button.Button(button.ButtonProps{Variant: button.Outline, Children: gx.Text("Three")})}),
	)},
}
