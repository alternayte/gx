package email

import "github.com/alternayte/gx"

// ColumnFixtures are the examples of Column in the dev gallery.
var ColumnFixtures = gx.Fixtures[ColumnProps]{
	"Default": {Children: gx.Text("A column shares the width of the row.")},
	"Fixed":   {Width: 160, Children: gx.Text("160 pixels")},
	"Right":   {Align: Right, Children: gx.Text("To the right")},
}

// ColumnWrap renders the column inside a row, as an email uses it.
func ColumnWrap(n gx.Node) gx.Node {
	return Row(RowProps{Children: n})
}
