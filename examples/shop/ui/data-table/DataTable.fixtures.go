package datatable

import (
	"strconv"

	"github.com/alternayte/gx"
)

// FixtureRow is the row type of the data table fixtures.
type FixtureRow struct {
	Name  string
	Total int
}

// FixtureColumns are the columns of the data table fixtures. The name column
// is a sort link.
var FixtureColumns = []Column[FixtureRow]{
	{Key: "name", Label: "Name", Sort: "name", Cell: func(r FixtureRow) gx.Node { return gx.Text(r.Name) }},
	{Key: "total", Label: "Total", Cell: func(r FixtureRow) gx.Node { return gx.Text(strconv.Itoa(r.Total)) }},
}

// FixtureHref is the Href of the data table fixtures. An app returns its own
// typed route here.
func FixtureHref(sort, dir string, page int) gx.URL { return gx.URL("/") }

var DataTableFixtures = gx.Fixtures[DataTableProps[FixtureRow]]{
	"Three": {
		Columns: FixtureColumns,
		Rows:    []FixtureRow{{Name: "Alpha", Total: 10}, {Name: "Beta", Total: 20}, {Name: "Gamma", Total: 30}},
		Page:    Page{Number: 1, Size: 10, Total: 3},
		Href:    FixtureHref,
	},
	"Empty": {
		Columns: FixtureColumns,
		Page:    Page{Number: 1, Size: 10, Total: 0},
		Href:    FixtureHref,
	},
}
