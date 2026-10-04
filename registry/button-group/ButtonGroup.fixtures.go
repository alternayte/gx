package buttongroup

import (
	"github.com/alternayte/gx"
	"github.com/alternayte/gx/registry/button"
)

var ButtonGroupFixtures = gx.Fixtures[ButtonGroupProps]{
	"Three": {Children: gx.Frag(
		button.Button(button.ButtonProps{Variant: button.Outline, Children: gx.Text("One")}),
		button.Button(button.ButtonProps{Variant: button.Outline, Children: gx.Text("Two")}),
		button.Button(button.ButtonProps{Variant: button.Outline, Children: gx.Text("Three")}),
	)},
}
