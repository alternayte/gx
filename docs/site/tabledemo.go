package site

import (
	"fmt"
	"sort"

	"github.com/alternayte/gx"
	"github.com/alternayte/gx/docs/site/route"
	datatable "github.com/alternayte/gx/registry/data-table"
	datatablepage "github.com/alternayte/gx/registry/data-table-page"
)

// invoice is one row of the live data table example.
type invoice struct {
	ID       string
	Customer string
	Status   string
	Cents    int
}

// invoices is the data of the live data table example.
var invoices = []invoice{
	{"INV-001", "Northwind", "Paid", 12000}, {"INV-002", "Acme", "Open", 8000},
	{"INV-003", "Globex", "Paid", 31000}, {"INV-004", "Initech", "Overdue", 4500},
	{"INV-005", "Umbrella", "Open", 22050}, {"INV-006", "Hooli", "Paid", 9900},
	{"INV-007", "Stark", "Paid", 150000}, {"INV-008", "Wayne", "Open", 64000},
	{"INV-009", "Wonka", "Overdue", 1250}, {"INV-010", "Tyrell", "Paid", 77700},
	{"INV-011", "Cyberdyne", "Open", 5400}, {"INV-012", "Soylent", "Paid", 2300},
	{"INV-013", "Aperture", "Overdue", 18800}, {"INV-014", "Vandelay", "Paid", 41000},
	{"INV-015", "Oscorp", "Open", 9950}, {"INV-016", "Monarch", "Paid", 12800},
	{"INV-017", "Dunder", "Paid", 3600}, {"INV-018", "Pied Piper", "Open", 27500},
	{"INV-019", "Gringotts", "Overdue", 86000}, {"INV-020", "Massive", "Paid", 15400},
	{"INV-021", "Virtucon", "Open", 6100}, {"INV-022", "Zorg", "Paid", 49900},
	{"INV-023", "Bluth", "Overdue", 720},
}

// tableDemoSize is the page size of the live data table example.
const tableDemoSize = 5

// tableDemoStatuses are the filter values, with their labels.
var tableDemoStatuses = [][2]string{{"all", "All"}, {"paid", "Paid"}, {"open", "Open"}, {"overdue", "Overdue"}}

// tableDemoSorts are the sort values of the columns. "none" is the order of
// the data.
var tableDemoSorts = []string{"none", "invoice", "customer", "amount"}

// tableDemoItems are the items that show the example.
var tableDemoItems = []string{"data-table", "data-table-page"}

// tableDemoColumns are the columns of the live data table example.
var tableDemoColumns = []datatable.Column[invoice]{
	{Key: "invoice", Label: "Invoice", Sort: "invoice", Cell: func(r invoice) gx.Node { return gx.Text(r.ID) }},
	{Key: "customer", Label: "Customer", Sort: "customer", Cell: func(r invoice) gx.Node { return gx.Text(r.Customer) }},
	{Key: "status", Label: "Status", Cell: func(r invoice) gx.Node { return gx.Text(r.Status) }},
	{Key: "amount", Label: "Amount", Sort: "amount", Cell: func(r invoice) gx.Node {
		return gx.Text(fmt.Sprintf("$%d.%02d", r.Cents/100, r.Cents%100))
	}},
}

// tableDemoRows returns the rows of one filter and sort, before paging. It
// does what the query of an app does.
func tableDemoRows(status, by, dir string) []invoice {
	var rows []invoice
	for _, r := range invoices {
		if status == "all" || statusValue(r.Status) == status {
			rows = append(rows, r)
		}
	}
	less := map[string]func(a, b invoice) bool{
		"invoice":  func(a, b invoice) bool { return a.ID < b.ID },
		"customer": func(a, b invoice) bool { return a.Customer < b.Customer },
		"amount":   func(a, b invoice) bool { return a.Cents < b.Cents },
	}[by]
	if less != nil {
		sort.SliceStable(rows, func(i, j int) bool {
			if dir == "desc" {
				return less(rows[j], rows[i])
			}
			return less(rows[i], rows[j])
		})
	}
	return rows
}

// statusValue returns the filter value of a status label.
func statusValue(label string) string {
	for _, s := range tableDemoStatuses {
		if s[1] == label {
			return s[0]
		}
	}
	return ""
}

// tableDemoPages returns the number of pages of a row count.
func tableDemoPages(rows int) int {
	if rows == 0 {
		return 1
	}
	return (rows + tableDemoSize - 1) / tableDemoSize
}

// tableDemoValid reports whether in is a state of the example.
func tableDemoValid(in route.TableDemo) bool {
	ok := false
	for _, item := range tableDemoItems {
		ok = ok || item == in.Item
	}
	if !ok || statusLabel(in.Status) == "" {
		return false
	}
	ok = false
	for _, s := range tableDemoSorts {
		ok = ok || s == in.Sort
	}
	if !ok || (in.Dir != "asc" && in.Dir != "desc") || (in.Sort == "none" && in.Dir != "asc") {
		return false
	}
	return in.Page >= 1 && in.Page <= tableDemoPages(len(tableDemoRows(in.Status, in.Sort, in.Dir)))
}

// statusLabel returns the label of a filter value, or the empty string.
func statusLabel(value string) string {
	for _, s := range tableDemoStatuses {
		if s[0] == value {
			return s[1]
		}
	}
	return ""
}

// tableDemoStart is the first state of the example of an item.
func tableDemoStart(item string) route.TableDemo {
	return route.TableDemo{Item: item, Status: "all", Sort: "none", Dir: "asc", Page: 1}
}

// TableDemoPage renders one state of the live data table example. The
// static export writes each state.
var TableDemoPage = gx.Page(
	func(c *gx.Ctx, in route.TableDemo) (PreviewProps, error) {
		if !tableDemoValid(in) {
			return PreviewProps{}, gx.NotFound()
		}
		all := tableDemoRows(in.Status, in.Sort, in.Dir)
		from := (in.Page - 1) * tableDemoSize
		to := min(from+tableDemoSize, len(all))
		by := in.Sort
		if by == "none" {
			by = ""
		}
		table := datatable.DataTable(datatable.DataTableProps[invoice]{
			Columns: tableDemoColumns,
			Rows:    all[from:to],
			Page:    datatable.Page{Number: in.Page, Size: tableDemoSize, Total: len(all)},
			Sort:    by,
			Dir:     in.Dir,
			Href: func(sortBy, dir string, page int) gx.URL {
				if sortBy == "" {
					sortBy, dir = "none", "asc"
				}
				return gx.URL(route.TableDemo{Item: in.Item, Status: in.Status, Sort: sortBy, Dir: dir, Page: page}.URL())
			},
			Empty: gx.Text("No invoice has this status."),
		})
		var filters []TableDemoFilter
		for _, s := range tableDemoStatuses {
			// A new filter starts at page 1 and keeps the sort.
			filters = append(filters, TableDemoFilter{
				Label:   s[1],
				Href:    route.TableDemo{Item: in.Item, Status: s[0], Sort: in.Sort, Dir: in.Dir, Page: 1},
				Current: s[0] == in.Status,
			})
		}
		view := TableDemoView(TableDemoViewProps{Filters: filters, Table: table})
		block := in.Item == "data-table-page"
		if block {
			view = datatablepage.DataTablePage(datatablepage.DataTablePageProps{
				Title:       "Invoices",
				Description: "Every invoice for this workspace.",
				Table:       view,
			})
		}
		return PreviewProps{Title: "Data Table: live example", Block: block, Children: view}, nil
	},
	Preview,
).Static(func() ([]route.TableDemo, error) {
	var out []route.TableDemo
	for _, item := range tableDemoItems {
		for _, s := range tableDemoStatuses {
			for _, by := range tableDemoSorts {
				for _, dir := range []string{"asc", "desc"} {
					if by == "none" && dir == "desc" {
						continue
					}
					for page := 1; page <= tableDemoPages(len(tableDemoRows(s[0], by, dir))); page++ {
						out = append(out, route.TableDemo{Item: item, Status: s[0], Sort: by, Dir: dir, Page: page})
					}
				}
			}
		}
	}
	return out, nil
})

// TableDemoFilter is one filter link of the live data table example.
type TableDemoFilter struct {
	Label   string
	Href    route.TableDemo
	Current bool
}
