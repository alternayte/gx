package field

import "github.com/alternayte/gx"

var FieldLegendFixtures = gx.Fixtures[FieldLegendProps]{
	"Legend": {Children: gx.Text("Address")},
	"Label":  {Variant: Label, Children: gx.Text("Address")},
}

// FieldLegendWrap renders the legend in a field set, where a legend
// belongs.
func FieldLegendWrap(n gx.Node) gx.Node {
	return FieldSet(FieldSetProps{Children: n})
}
