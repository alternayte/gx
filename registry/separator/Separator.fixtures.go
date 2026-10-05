package separator

import "github.com/alternayte/gx"

var SeparatorFixtures = gx.Fixtures[SeparatorProps]{
	"Horizontal": {Decorative: true, Class: "w-40"},
	"Vertical":   {Decorative: true, Orientation: Vertical, Class: "h-8"},
	"Semantic":   {Class: "w-40"},
}
