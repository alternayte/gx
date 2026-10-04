package selectbox

import "github.com/alternayte/gx"

var SelectOptionFixtures = gx.Fixtures[SelectOptionProps]{"Option": {Value: "free", Children: gx.Text("Free")}}
