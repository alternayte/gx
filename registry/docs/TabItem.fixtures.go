package docs

import "github.com/alternayte/gx"

var TabItemFixtures = gx.Fixtures[TabItemProps]{
	"Text": {Label: "Tab", Children: gx.Text("Panel body.")},
}
