// Package datatable renders a server-driven table (REQ-REG-10). Sort,
// filter and paging state live in typed GET routes; the component only
// renders one page and links the next one.
package datatable

import (
	"strconv"

	"github.com/alternayte/gx"
	"github.com/alternayte/gx/registry/pagination"
	"github.com/alternayte/gx/registry/table"
)

// Column is one typed column of a data table.
type Column[T any] struct {
	// Key identifies the column in the row data.
	Key string
	// Label is the visible header text.
	Label string
	// Cell renders one cell.
	Cell func(T) gx.Node
	// Sort is the query value of the column when the header is a sort
	// link. Empty makes the column unsortable.
	Sort string
}

// Page is the paging state of one table render.
type Page struct {
	Number int
	Size   int
	Total  int
}

// Href builds the typed GET URL of one table state. The app returns its
// own route value, so the query stays typed.
type Href func(sort, dir string, page int) gx.URL

// DataTableProps holds one rendered table.
type DataTableProps[T any] struct {
	Columns []Column[T]
	Rows    []T
	Page    Page
	Sort    string
	Dir     string
	Href    Href
	// Empty renders in place of the rows when the page holds none.
	Empty gx.Node
}

// DataTable renders one page of a server-driven table with sortable
// headers and paging links (REQ-REG-10).
func DataTable[T any](p DataTableProps[T]) gx.Node {
	head := make([]gx.Node, 0, len(p.Columns))
	for _, col := range p.Columns {
		head = append(head, header(p, col))
	}
	body := make([]gx.Node, 0, len(p.Rows))
	for _, row := range p.Rows {
		cells := make([]gx.Node, 0, len(p.Columns))
		for _, col := range p.Columns {
			cells = append(cells, table.TableCell(table.TableCellProps{Children: col.Cell(row)}))
		}
		body = append(body, table.TableRow(table.TableRowProps{Children: gx.Frag(cells...)}))
	}
	var rows gx.Node
	switch {
	case len(p.Rows) > 0:
		rows = gx.Frag(body...)
	case p.Empty != nil:
		rows = table.TableRow(table.TableRowProps{
			Children: table.TableCell(table.TableCellProps{
				Class:    "h-24 text-center text-muted-foreground",
				Children: p.Empty,
			}),
		})
	default:
		rows = table.TableRow(table.TableRowProps{
			Children: table.TableCell(table.TableCellProps{
				Class:    "h-24 text-center text-muted-foreground",
				Children: gx.Text("No results."),
			}),
		})
	}
	parts := []gx.Node{
		table.Table(table.TableProps{Children: gx.Frag(
			table.TableHeader(table.TableHeaderProps{Children: table.TableRow(table.TableRowProps{Children: gx.Frag(head...)})}),
			table.TableBody(table.TableBodyProps{Children: rows}),
		)}),
	}
	if p.Page.Size > 0 {
		parts = append(parts, paging(p))
	}
	return gx.Frag(parts...)
}

// header renders one header cell, with a sort link when the column sorts.
func header[T any](p DataTableProps[T], col Column[T]) gx.Node {
	if col.Sort == "" {
		return table.TableHead(table.TableHeadProps{Children: gx.Text(col.Label)})
	}
	dir := "asc"
	arrow := "↕"
	attrs := gx.Attrs{}
	if p.Sort == col.Sort {
		switch p.Dir {
		case "desc":
			dir = "asc"
			arrow = "↓"
			attrs = append(attrs, gx.Attr{Key: "aria-sort", Value: "descending"})
		default:
			dir = "desc"
			arrow = "↑"
			attrs = append(attrs, gx.Attr{Key: "aria-sort", Value: "ascending"})
		}
	}
	link := gx.El("a", gx.Attrs{
		{Key: "href", Value: string(p.Href(col.Sort, dir, 1)), Kind: gx.AttrURL},
		{Key: "class", Value: "inline-flex items-center gap-1 no-underline hover:text-foreground"},
	}, gx.Text(col.Label), gx.El("span", gx.Attrs{{Key: "class", Value: "text-muted-foreground"}}, gx.Text(arrow)))
	return table.TableHead(table.TableHeadProps{Attrs: attrs, Children: link})
}

// paging renders the footer with the row range and the page links.
func paging[T any](p DataTableProps[T]) gx.Node {
	pages := (p.Page.Total + p.Page.Size - 1) / p.Page.Size
	if pages < 1 {
		pages = 1
	}
	number := p.Page.Number
	if number < 1 {
		number = 1
	}
	if number > pages {
		number = pages
	}
	from := (number-1)*p.Page.Size + 1
	to := from + len(p.Rows) - 1
	if p.Page.Total == 0 {
		from, to = 0, 0
	}
	items := []gx.Node{
		pagination.PaginationItem(pagination.PaginationItemProps{Children: pagination.PaginationPrevious(pagination.PaginationPreviousProps{
			Href:  p.Href(p.Sort, p.Dir, number-1),
			Class: previousClass(number),
		})}),
	}
	for _, page := range pageWindow(number, pages) {
		page := page
		if page == 0 {
			items = append(items, pagination.PaginationItem(pagination.PaginationItemProps{
				Children: pagination.PaginationEllipsis(pagination.PaginationEllipsisProps{}),
			}))
			continue
		}
		items = append(items, pagination.PaginationItem(pagination.PaginationItemProps{
			Children: pagination.PaginationLink(pagination.PaginationLinkProps{
				Href:     p.Href(p.Sort, p.Dir, page),
				Active:   page == number,
				Children: gx.Text(strconv.Itoa(page)),
			}),
		}))
	}
	items = append(items, pagination.PaginationItem(pagination.PaginationItemProps{Children: pagination.PaginationNext(pagination.PaginationNextProps{
		Href:  p.Href(p.Sort, p.Dir, number+1),
		Class: nextClass(number, pages),
	})}))
	return gx.El("div", gx.Attrs{{Key: "class", Value: "flex items-center justify-between gap-4 px-2 pt-4"}},
		gx.El("p", gx.Attrs{{Key: "class", Value: "text-sm text-muted-foreground"}},
			gx.Text(strconv.Itoa(from)+"–"+strconv.Itoa(to)+" of "+strconv.Itoa(p.Page.Total))),
		pagination.Pagination(pagination.PaginationProps{Children: pagination.PaginationContent(pagination.PaginationContentProps{Children: gx.Frag(items...)})}),
	)
}

// previousClass hides the previous link on the first page.
func previousClass(number int) string {
	if number <= 1 {
		return "pointer-events-none opacity-50"
	}
	return ""
}

// nextClass hides the next link on the last page.
func nextClass(number, pages int) string {
	if number >= pages {
		return "pointer-events-none opacity-50"
	}
	return ""
}

// pageWindow returns the page numbers to show, with 0 for an ellipsis.
func pageWindow(number, pages int) []int {
	if pages <= 7 {
		out := make([]int, 0, pages)
		for i := 1; i <= pages; i++ {
			out = append(out, i)
		}
		return out
	}
	out := []int{1}
	start, end := number-1, number+1
	if start < 2 {
		start, end = 2, 4
	}
	if end > pages-1 {
		start, end = pages-4, pages-1
	}
	if start > 2 {
		out = append(out, 0)
	}
	for i := start; i <= end; i++ {
		out = append(out, i)
	}
	if end < pages-1 {
		out = append(out, 0)
	}
	return append(out, pages)
}
