package toast

import "github.com/alternayte/gx"

var ToasterFixtures = gx.Fixtures[ToasterProps]{
	"Empty":     {},
	"WithToast": {Children: gx.ToastNode("Saved")},
}
