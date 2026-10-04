package field

import "github.com/alternayte/gx"

var FieldErrorFixtures = gx.Fixtures[FieldErrorProps]{"Error": {Children: gx.Text("This field is required.")}}
