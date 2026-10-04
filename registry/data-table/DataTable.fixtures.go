package datatable

import (
	"strconv"

	"github.com/alternayte/gx"
)

type fixtureRow struct {
	Name  string
	Total int
}

var DataTableFixtures = gx.Fixtures[DataTableProps[fixtureRow]]{
	"Three": {
		Columns: []Column[fixtureRow]{
			{Key: "name", Label: "Name", Sort: "name", Cell: func(r fixtureRow) gx.Node { return gx.Text(r.Name) }},
			{Key: "total", Label: "Total", Cell: func(r fixtureRow) gx.Node { return gx.Text(strconv.Itoa(r.Total)) }},
		},
		Rows: []fixtureRow{{Name: "Alpha", Total: 10}, {Name: "Beta", Total: 20}, {Name: "Gamma", Total: 30}},
		Page: Page{Number: 1, Size: 10, Total: 3},
		Href: func(sort, dir string, page int) gx.URL { return gx.URL("/") },
	},
	"Empty": {
		Columns: []Column[fixtureRow]{
			{Key: "name", Label: "Name", Cell: func(r fixtureRow) gx.Node { return gx.Text(r.Name) }},
		},
		Page: Page{Number: 1, Size: 10, Total: 0},
		Href: func(sort, dir string, page int) gx.URL { return gx.URL("/") },
	},
}
