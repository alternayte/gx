package tabs

import "github.com/alternayte/gx"

var TabItemFixtures = gx.Fixtures[TabItemProps]{
	"Item": {Label: "Account", Children: gx.Text("Panel.")},
}
