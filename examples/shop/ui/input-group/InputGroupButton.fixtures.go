package inputgroup

import (
	"github.com/alternayte/gx"
	"github.com/alternayte/gx/examples/shop/ui/icons"
)

var InputGroupButtonFixtures = gx.Fixtures[InputGroupButtonProps]{
	"ExtraSmall": {Children: gx.Text("Search")},
	"Small":      {Size: Sm, Children: gx.Text("Search")},
	"IconXs":     {Size: IconXs, Attrs: name("Clear"), Children: icons.X(icons.XProps{})},
	"IconSm":     {Size: IconSm, Attrs: name("Clear"), Children: icons.X(icons.XProps{})},
}
