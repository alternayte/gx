package scrollarea

import "github.com/alternayte/gx"

// Each line is one paragraph, so the text does not wrap at the scrollbar.
var ScrollAreaFixtures = gx.Fixtures[ScrollAreaProps]{
	"Vertical": {Class: "h-24 w-48 rounded-md border border-border", Attrs: gx.Attrs{{Key: "aria-label", Value: "Lines", Kind: gx.AttrText}}, Children: gx.El("div", gx.Attrs{{Key: "class", Value: "p-2 text-sm", Kind: gx.AttrText}},
		gx.El("p", nil, gx.Text("Line one.")),
		gx.El("p", nil, gx.Text("Line two.")),
		gx.El("p", nil, gx.Text("Line three.")),
		gx.El("p", nil, gx.Text("Line four.")),
		gx.El("p", nil, gx.Text("Line five.")),
		gx.El("p", nil, gx.Text("Line six.")),
		gx.El("p", nil, gx.Text("Line seven.")),
		gx.El("p", nil, gx.Text("Line eight.")),
	)},
	"Horizontal": {Class: "w-48 rounded-md border border-border whitespace-nowrap", Attrs: gx.Attrs{{Key: "aria-label", Value: "One long line", Kind: gx.AttrText}}, Children: gx.El("div", gx.Attrs{{Key: "class", Value: "p-2 text-sm", Kind: gx.AttrText}},
		gx.El("p", nil, gx.Text("One long line that does not wrap and scrolls sideways.")),
	)},
}
