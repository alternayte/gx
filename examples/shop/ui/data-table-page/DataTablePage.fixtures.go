package datatablepage

import (
	"github.com/alternayte/gx"
	"github.com/alternayte/gx/examples/shop/ui/input"
	"github.com/alternayte/gx/examples/shop/ui/pagination"
	"github.com/alternayte/gx/examples/shop/ui/table"
)

// row renders one invoice row of the fixture table.
func row(id, status, amount string) gx.Node {
	return table.TableRow(table.TableRowProps{Children: gx.Frag(
		table.TableCell(table.TableCellProps{Class: "font-medium", Children: gx.Text(id)}),
		table.TableCell(table.TableCellProps{Children: gx.Text(status)}),
		table.TableCell(table.TableCellProps{Class: "text-right", Children: gx.Text(amount)}),
	)})
}

var DataTablePageFixtures = gx.Fixtures[DataTablePageProps]{
	"Default": {
		Title:       "Invoices",
		Description: "Every invoice for this workspace.",
		Filter:      input.Input(input.InputProps{Name: "q", Type: "search", Placeholder: "Search", Attrs: gx.Attrs{{Key: "aria-label", Value: "Search"}}}),
		Table: table.Table(table.TableProps{Children: gx.Frag(
			table.TableHeader(table.TableHeaderProps{Children: table.TableRow(table.TableRowProps{Children: gx.Frag(
				table.TableHead(table.TableHeadProps{Children: gx.Text("Invoice")}),
				table.TableHead(table.TableHeadProps{Children: gx.Text("Status")}),
				table.TableHead(table.TableHeadProps{Class: "text-right", Children: gx.Text("Amount")}),
			)})}),
			table.TableBody(table.TableBodyProps{Children: gx.Frag(
				row("INV-001", "Paid", "$120.00"),
				row("INV-002", "Open", "$80.00"),
				row("INV-003", "Paid", "$310.00"),
			)}),
		)}),
		Pagination: pagination.Pagination(pagination.PaginationProps{Children: pagination.PaginationContent(pagination.PaginationContentProps{Children: gx.Frag(
			pagination.PaginationItem(pagination.PaginationItemProps{Children: pagination.PaginationPrevious(pagination.PaginationPreviousProps{Href: gx.URL("/invoices?page=1")})}),
			pagination.PaginationItem(pagination.PaginationItemProps{Children: pagination.PaginationLink(pagination.PaginationLinkProps{Href: gx.URL("/invoices?page=1"), Active: true, Children: gx.Text("1")})}),
			pagination.PaginationItem(pagination.PaginationItemProps{Children: pagination.PaginationLink(pagination.PaginationLinkProps{Href: gx.URL("/invoices?page=2"), Children: gx.Text("2")})}),
			pagination.PaginationItem(pagination.PaginationItemProps{Children: pagination.PaginationNext(pagination.PaginationNextProps{Href: gx.URL("/invoices?page=2")})}),
		)})}),
	},
}
