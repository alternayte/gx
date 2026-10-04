package table

import "github.com/alternayte/gx"

var TableCellFixtures = gx.Fixtures[TableCellProps]{"Cell": {Children: gx.Text("Value")}}
