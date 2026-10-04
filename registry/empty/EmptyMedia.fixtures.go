package empty

import "github.com/alternayte/gx"

var EmptyMediaFixtures = gx.Fixtures[EmptyMediaProps]{"Icon": {Variant: Icon, Children: gx.Text("+")}}
