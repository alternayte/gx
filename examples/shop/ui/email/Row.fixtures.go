package email

import "github.com/alternayte/gx"

// RowFixtures are the examples of Row in the dev gallery.
var RowFixtures = gx.Fixtures[RowProps]{
	"TwoColumns": {Children: gx.Frag(
		Column(ColumnProps{Children: gx.Text("Tea, 2 boxes")}),
		Column(ColumnProps{Width: 120, Align: Right, Children: gx.Text("12.00")}),
	)},
}
