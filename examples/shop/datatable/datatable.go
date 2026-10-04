// Package datatable demos the server-driven data table (REQ-REG-10).
package datatable

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/alternayte/gx"
	"github.com/alternayte/gx/examples/shop/datatable/route"
	datatable "github.com/alternayte/gx/examples/shop/ui/data-table"
	datatablepage "github.com/alternayte/gx/examples/shop/ui/data-table-page"
)

// PageSize is the number of rows on one page.
const PageSize = 25

// RowCount is the size of the demo set.
const RowCount = 10000

// Row is one demo row.
type Row struct {
	Name  string
	Total int
}

// Rows is the fixed 10,000 row set.
var Rows = makeRows()

// Columns are the typed columns of the table.
var Columns = []datatable.Column[Row]{
	{Key: "name", Label: "Name", Sort: "name", Cell: func(r Row) gx.Node { return gx.Text(r.Name) }},
	{Key: "total", Label: "Total", Sort: "total", Cell: func(r Row) gx.Node { return gx.Text(strconv.Itoa(r.Total)) }},
}

func makeRows() []Row {
	out := make([]Row, 0, RowCount)
	for i := 0; i < RowCount; i++ {
		out = append(out, Row{Name: fmt.Sprintf("Item %05d", i), Total: (i * 37) % 1000})
	}
	return out
}

// ListPage serves one page of the table.
var ListPage = gx.Page(load, view)

// tableProps is the loaded page data.
type tableProps struct {
	Rows  []Row
	In    route.List
	Total int
}

// load filters, sorts and pages the rows with the typed query values.
func load(c *gx.Ctx, in route.List) (tableProps, error) {
	rows := append([]Row(nil), Rows...)
	if in.Q != "" {
		q := strings.ToLower(in.Q)
		filtered := make([]Row, 0, len(rows))
		for _, row := range rows {
			if strings.Contains(strings.ToLower(row.Name), q) {
				filtered = append(filtered, row)
			}
		}
		rows = filtered
	}
	byName := in.Sort != "total"
	asc := in.Dir != "desc"
	sort.SliceStable(rows, func(i, j int) bool {
		less := rows[i].Name < rows[j].Name
		if !byName {
			less = rows[i].Total < rows[j].Total
		}
		if asc {
			return less
		}
		return !less
	})
	total := len(rows)
	page := in.Page
	if page < 1 {
		page = 1
	}
	start := (page - 1) * PageSize
	if start > total {
		start = total
	}
	end := start + PageSize
	if end > total {
		end = total
	}
	return tableProps{Rows: rows[start:end], In: in, Total: total}, nil
}

// view renders the table inside the data table page block.
func view(p tableProps) gx.Node {
	filter := gx.El("form", gx.Attrs{
		{Key: "method", Value: "get"},
		{Key: "action", Value: route.List{}.URL()},
		{Key: "class", Value: "flex gap-2"},
	}, gx.El("input", gx.Attrs{
		{Key: "type", Value: "search"},
		{Key: "name", Value: "q"},
		{Key: "value", Value: p.In.Q},
		{Key: "placeholder", Value: "Filter by name"},
		{Key: "aria-label", Value: "Filter by name"},
		{Key: "class", Value: "h-9 w-full rounded-md border border-input bg-transparent px-3 text-sm"},
	}))
	return datatablepage.DataTablePage(datatablepage.DataTablePageProps{
		Title:       "Data table",
		Description: fmt.Sprintf("%d rows with server sort, filter and paging", p.Total),
		Filter:      filter,
		Table: datatable.DataTable(datatable.DataTableProps[Row]{
			Columns: Columns,
			Rows:    p.Rows,
			Page:    datatable.Page{Number: p.In.Page, Size: PageSize, Total: p.Total},
			Sort:    p.In.Sort,
			Dir:     p.In.Dir,
			Href: func(sortBy, dir string, page int) gx.URL {
				return gx.URL(route.List{Sort: sortBy, Dir: dir, Page: page, Q: p.In.Q}.URL())
			},
		}),
	})
}

// Routes collects the data table page.
var Routes = gx.Collect(ListPage)
