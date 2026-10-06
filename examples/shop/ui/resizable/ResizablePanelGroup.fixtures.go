package resizable

import "github.com/alternayte/gx"

// paneClass centres the text of a demo panel.
const paneClass = "flex items-center justify-center p-4 text-sm font-medium"

var ResizablePanelGroupFixtures = gx.Fixtures[ResizablePanelGroupProps]{
	"Horizontal": {Class: "h-40 max-w-md rounded-md border border-border", Children: gx.Frag(
		ResizablePanel(ResizablePanelProps{Size: 40, MinSize: 20, Class: paneClass, Children: gx.Text("One")}),
		ResizableHandle(ResizableHandleProps{Id: "resize-h"}),
		ResizablePanel(ResizablePanelProps{Size: 60, MinSize: 30, Class: paneClass, Children: gx.Text("Two")}),
	)},
	"Vertical": {Orientation: Vertical, Class: "h-56 max-w-md rounded-md border border-border", Children: gx.Frag(
		ResizablePanel(ResizablePanelProps{Size: 50, Class: paneClass, Children: gx.Text("Top")}),
		ResizableHandle(ResizableHandleProps{Id: "resize-v", Orientation: Vertical}),
		ResizablePanel(ResizablePanelProps{Size: 50, Class: paneClass, Children: gx.Text("Bottom")}),
	)},
}

// ResizablePanelGroupWrap gives a fixture the width of a page column.
func ResizablePanelGroupWrap(n gx.Node) gx.Node {
	return gx.El("div", gx.Attrs{{Key: "class", Value: "w-full max-w-md"}}, n)
}
