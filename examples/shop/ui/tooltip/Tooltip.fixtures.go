package tooltip

import "github.com/alternayte/gx"

var TooltipFixtures = gx.Fixtures[TooltipProps]{
	"Top":    {Content: "Add to library", Children: gx.Text("Hover me")},
	"Right":  {Content: "Add to library", Side: Right, Children: gx.Text("Hover me")},
	"Bottom": {Content: "Add to library", Side: Bottom, Children: gx.Text("Hover me")},
	"Left":   {Content: "Add to library", Side: Left, Children: gx.Text("Hover me")},
}
