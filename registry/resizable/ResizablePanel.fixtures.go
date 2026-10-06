package resizable

import "github.com/alternayte/gx"

var ResizablePanelFixtures = gx.Fixtures[ResizablePanelProps]{
	"Default": {Children: gx.Text("A panel")},
}
