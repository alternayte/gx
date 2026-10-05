package selectbox

import "github.com/alternayte/gx"

var SelectOptionFixtures = gx.Fixtures[SelectOptionProps]{"Option": {Value: "free", Children: gx.Text("Free")}}

// SelectOptionWrap renders the option inside a select, as a page uses it.
func SelectOptionWrap(n gx.Node) gx.Node {
	return Select(SelectProps{Label: "Plan", Children: n})
}
