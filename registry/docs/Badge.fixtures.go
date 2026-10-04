package docs

import "github.com/alternayte/gx"

var BadgeFixtures = gx.Fixtures[BadgeProps]{
	"Default":     {Label: "Default"},
	"Secondary":   {Label: "Secondary", Variant: BadgeSecondary},
	"Destructive": {Label: "Destructive", Variant: BadgeDestructive},
	"Outline":     {Label: "Outline", Variant: BadgeOutline},
}
