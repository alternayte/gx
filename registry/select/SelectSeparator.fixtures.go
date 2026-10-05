package selectbox

import "github.com/alternayte/gx"

var SelectSeparatorFixtures = gx.Fixtures[SelectSeparatorProps]{"Default": {}}

// SelectSeparatorWrap renders the separator between two options of a select.
func SelectSeparatorWrap(n gx.Node) gx.Node {
	return Select(SelectProps{Label: "Plan", Children: gx.Frag(
		SelectOption(SelectOptionProps{Value: "free", Children: gx.Text("Free")}),
		n,
		SelectOption(SelectOptionProps{Value: "pro", Children: gx.Text("Pro")}),
	)})
}
