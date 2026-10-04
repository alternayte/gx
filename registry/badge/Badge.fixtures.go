package badge

import "github.com/alternayte/gx"

var BadgeFixtures = gx.Fixtures[BadgeProps]{
	"Default":     {Children: gx.Text("Badge")},
	"Secondary":   {Variant: Secondary, Children: gx.Text("Secondary")},
	"Destructive": {Variant: Destructive, Children: gx.Text("Destructive")},
	"Outline":     {Variant: Outline, Children: gx.Text("Outline")},
}
