package field

import "github.com/alternayte/gx"

var FieldLabelFixtures = gx.Fixtures[FieldLabelProps]{"Label": {For: "email", Children: gx.Text("Email")}}
