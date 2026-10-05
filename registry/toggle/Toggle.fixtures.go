package toggle

import "github.com/alternayte/gx"

var ToggleFixtures = gx.Fixtures[ToggleProps]{
	"Off":       {Name: "bold", Children: gx.Text("Bold")},
	"On":        {Name: "bold", Pressed: true, Children: gx.Text("Bold")},
	"Disabled":  {Name: "bold", Disabled: true, Children: gx.Text("Bold")},
	"Outline":   {Name: "bold", Variant: Outline, Children: gx.Text("Bold")},
	"OutlineOn": {Name: "bold", Variant: Outline, Pressed: true, Children: gx.Text("Bold")},
	"Invalid":   {Name: "bold", Variant: Outline, Invalid: true, Children: gx.Text("Bold")},
	"Small":     {Name: "bold", Variant: Outline, Size: Sm, Children: gx.Text("Bold")},
	"Large":     {Name: "bold", Variant: Outline, Size: Lg, Children: gx.Text("Bold")},
}
