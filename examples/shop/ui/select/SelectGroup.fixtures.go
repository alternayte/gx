package selectbox

import "github.com/alternayte/gx"

var SelectGroupFixtures = gx.Fixtures[SelectGroupProps]{
	"Group": {Label: "Fruits", Children: gx.Frag(
		SelectOption(SelectOptionProps{Value: "apple", Children: gx.Text("Apple")}),
		SelectOption(SelectOptionProps{Value: "banana", Children: gx.Text("Banana")}),
	)},
}

// SelectGroupWrap renders the group inside a select, as a page uses it.
func SelectGroupWrap(n gx.Node) gx.Node {
	return Select(SelectProps{Label: "Fruit", Children: n})
}
