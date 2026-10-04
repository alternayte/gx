package label

import "github.com/alternayte/gx"

var LabelFixtures = gx.Fixtures[LabelProps]{
	"Plain": {For: "email", Children: gx.Text("Email")},
}
