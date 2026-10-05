package button

import (
	"github.com/alternayte/gx"
	"github.com/alternayte/gx/registry/icons"
)

// iconLabel is the accessible name of an icon-only fixture.
var iconLabel = gx.Attrs{{Key: "aria-label", Value: "Next"}}

var ButtonFixtures = gx.Fixtures[ButtonProps]{
	"Default":     {Children: gx.Text("Button")},
	"Secondary":   {Variant: Secondary, Children: gx.Text("Secondary")},
	"Destructive": {Variant: Destructive, Children: gx.Text("Delete")},
	"Outline":     {Variant: Outline, Children: gx.Text("Outline")},
	"Ghost":       {Variant: Ghost, Children: gx.Text("Ghost")},
	"Link":        {Variant: Link, Children: gx.Text("Link")},
	"ExtraSmall":  {Size: Xs, Children: gx.Text("Extra small")},
	"Small":       {Size: Sm, Children: gx.Text("Small")},
	"Large":       {Size: Lg, Children: gx.Text("Large")},
	"WithIcon":    {Variant: Outline, Children: gx.Frag(gx.Text("Next"), icons.ChevronRight(icons.ChevronRightProps{}))},
	"Icon":        {Variant: Outline, Size: Icon, Attrs: iconLabel, Children: icons.ChevronRight(icons.ChevronRightProps{})},
	"IconXs":      {Variant: Outline, Size: IconXs, Attrs: iconLabel, Children: icons.ChevronRight(icons.ChevronRightProps{})},
	"IconSm":      {Variant: Outline, Size: IconSm, Attrs: iconLabel, Children: icons.ChevronRight(icons.ChevronRightProps{})},
	"IconLg":      {Variant: Outline, Size: IconLg, Attrs: iconLabel, Children: icons.ChevronRight(icons.ChevronRightProps{})},
	"Disabled":    {Attrs: gx.Attrs{gx.Bool("disabled", true)}, Children: gx.Text("Disabled")},
}
