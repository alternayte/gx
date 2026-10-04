package toast

import "github.com/alternayte/gx"

var ToasterFixtures = gx.Fixtures[ToasterProps]{
	"WithToast": {Children: gx.ToastNode("Saved")},
}
