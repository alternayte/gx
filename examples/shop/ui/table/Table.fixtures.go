package table

import "github.com/alternayte/gx"

// invoiceRows are the body rows of the table fixtures.
func invoiceRows() gx.Node {
	return gx.Frag(
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
	)
}

// invoiceHead is the header row of the table fixtures.
func invoiceHead() gx.Node {
	return TableHeader(TableHeaderProps{Children: TableRow(TableRowProps{Children: gx.Frag(
		TableHead(TableHeadProps{Children: gx.Text("Invoice")}),
		TableHead(TableHeadProps{Children: gx.Text("Status")}),
		TableHead(TableHeadProps{Children: gx.Text("Amount")}),
	)})})
}

var TableFixtures = gx.Fixtures[TableProps]{
	"Simple": {Children: gx.Frag(
		TableCaption(TableCaptionProps{Children: gx.Text("A list of invoices.")}),
		invoiceHead(),
		TableBody(TableBodyProps{Children: invoiceRows()}),
	)},
	"Footer": {Children: gx.Frag(
		invoiceHead(),
		TableBody(TableBodyProps{Children: invoiceRows()}),
		TableFooter(TableFooterProps{Children: TableRow(TableRowProps{Children: gx.Frag(
			TableCell(TableCellProps{Attrs: gx.Attrs{{Key: "colspan", Value: "2"}}, Children: gx.Text("Total")}),
			TableCell(TableCellProps{Children: gx.Text("$200.00")}),
		)})}),
	)},
	"Selected": {Children: gx.Frag(
		invoiceHead(),
		TableBody(TableBodyProps{Children: gx.Frag(
			TableRow(TableRowProps{Attrs: gx.Attrs{{Key: "data-state", Value: "selected"}}, Children: gx.Frag(
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
