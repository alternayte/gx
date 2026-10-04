package table

import "github.com/alternayte/gx"

var TableHeadFixtures = gx.Fixtures[TableHeadProps]{"Header": {Children: gx.Text("Column")}}
