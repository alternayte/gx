package field

import "github.com/alternayte/gx"

var FieldErrorFixtures = gx.Fixtures[FieldErrorProps]{
	"Error": {Children: gx.Text("This field is required.")},
	"One":   {Errors: []string{"Enter a valid email address.", "Enter a valid email address."}},
	"List":  {Errors: []string{"Use 12 characters or more.", "Use one number."}},
}
