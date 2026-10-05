package scrollarea

import "github.com/alternayte/gx"

// lines returns one paragraph per line, so the text does not wrap at the
// scrollbar.
func lines(text ...string) gx.Node {
	nodes := make([]gx.Node, 0, len(text))
	for _, line := range text {
		nodes = append(nodes, gx.El("p", nil, gx.Text(line)))
	}
	return gx.El("div", gx.Attrs{{Key: "class", Value: "p-2 text-sm", Kind: gx.AttrText}}, nodes...)
}

var ScrollAreaFixtures = gx.Fixtures[ScrollAreaProps]{
	"Vertical": {Class: "h-24 w-48 rounded-md border border-border", Attrs: gx.Attrs{{Key: "aria-label", Value: "Lines", Kind: gx.AttrText}}, Children: lines("Line one.", "Line two.", "Line three.", "Line four.", "Line five.", "Line six.", "Line seven.", "Line eight.")},
	"Horizontal": {Class: "w-48 rounded-md border border-border whitespace-nowrap", Attrs: gx.Attrs{{Key: "aria-label", Value: "One long line", Kind: gx.AttrText}}, Children: lines("One long line that does not wrap and scrolls sideways.")},
}
