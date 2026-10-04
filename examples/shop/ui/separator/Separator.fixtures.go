package separator

import "github.com/alternayte/gx"

var SeparatorFixtures = gx.Fixtures[SeparatorProps]{
	"Horizontal": {Class: "w-40"},
	"Vertical":   {Orientation: Vertical, Class: "h-8"},
}
