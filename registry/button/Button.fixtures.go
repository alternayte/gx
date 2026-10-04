package button

import "github.com/alternayte/gx"

var ButtonFixtures = gx.Fixtures[ButtonProps]{
	"Default":     {Children: gx.Text("Button")},
	"Secondary":   {Variant: Secondary, Children: gx.Text("Secondary")},
	"Destructive": {Variant: Destructive, Children: gx.Text("Delete")},
	"Outline":     {Variant: Outline, Children: gx.Text("Outline")},
	"Ghost":       {Variant: Ghost, Children: gx.Text("Ghost")},
	"Link":        {Variant: Link, Children: gx.Text("Link")},
	"Small":       {Size: Sm, Children: gx.Text("Small")},
	"Large":       {Size: Lg, Children: gx.Text("Large")},
	"Icon":        {Size: Icon, Children: gx.Text("+")},
	"Disabled":    {Attrs: gx.Attrs{gx.Bool("disabled", true)}, Children: gx.Text("Disabled")},
}
