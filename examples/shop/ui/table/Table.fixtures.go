package table

import "github.com/alternayte/gx"

var TableFixtures = gx.Fixtures[TableProps]{
	"Simple": {Children: gx.Frag(
		TableCaption(TableCaptionProps{Children: gx.Text("A list of invoices.")}),
		TableHeader(TableHeaderProps{Children: TableRow(TableRowProps{Children: gx.Frag(
			TableHead(TableHeadProps{Children: gx.Text("Invoice")}),
			TableHead(TableHeadProps{Children: gx.Text("Status")}),
			TableHead(TableHeadProps{Children: gx.Text("Amount")}),
		)})}),
		TableBody(TableBodyProps{Children: gx.Frag(
			TableRow(TableRowProps{Children: gx.Frag(
				TableCell(TableCellProps{Children: gx.Text("INV-001")}),
				TableCell(TableCellProps{Children: gx.Text("Paid")}),
				TableCell(TableCellProps{Children: gx.Text("$120.00")}),
			)}),
			TableRow(TableRowProps{Children: gx.Frag(
				TableCell(TableCellProps{Children: gx.Text("INV-002")}),
				TableCell(TableCellProps{Children: gx.Text("Open")}),
				TableCell(TableCellProps{Children: gx.Text("$80.00")}),
			)}),
		)}),
	)},
}
