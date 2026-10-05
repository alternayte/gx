package buttongroup

import (
	"github.com/alternayte/gx"
	"github.com/alternayte/gx/examples/shop/ui/button"
)

var ButtonGroupSeparatorFixtures = gx.Fixtures[ButtonGroupSeparatorProps]{"Default": {}}

// ButtonGroupSeparatorWrap renders the separator between two buttons, as a
// group uses it.
func ButtonGroupSeparatorWrap(n gx.Node) gx.Node {
	return ButtonGroup(ButtonGroupProps{Children: gx.Frag(
		button.Button(button.ButtonProps{Variant: button.Secondary, Children: gx.Text("One")}),
		n,
		button.Button(button.ButtonProps{Variant: button.Secondary, Children: gx.Text("Two")}),
	)})
}
