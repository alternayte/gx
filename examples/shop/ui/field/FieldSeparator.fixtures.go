package field

import "github.com/alternayte/gx"

var FieldSeparatorFixtures = gx.Fixtures[FieldSeparatorProps]{
	"Line": {},
	"Text": {Children: gx.Text("Or continue with")},
}
