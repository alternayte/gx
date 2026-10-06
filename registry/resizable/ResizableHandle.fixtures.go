package resizable

import "github.com/alternayte/gx"

var ResizableHandleFixtures = gx.Fixtures[ResizableHandleProps]{
	"Between": {Id: "resize-between"},
}

// ResizableHandleWrap puts the handle between two panels: a handle works
// only there.
func ResizableHandleWrap(n gx.Node) gx.Node {
	return ResizablePanelGroup(ResizablePanelGroupProps{Class: "h-24 max-w-md rounded-md border border-border", Children: gx.Frag(
		ResizablePanel(ResizablePanelProps{Size: 50, Children: gx.Text("One")}),
		n,
		ResizablePanel(ResizablePanelProps{Size: 50, Children: gx.Text("Two")}),
	)})
}
